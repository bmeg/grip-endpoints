/*
GraphQL Web endpoint
*/

package main

import (
	"fmt"
	"net/http"
    "sync"
    "time"
    "io"
    "encoding/json"
    "errors"

	"github.com/bmeg/grip/gripql"
	"github.com/bmeg/grip/log"
	"github.com/graphql-go/handler"
)

 type UserAuth struct {
     ExpiresAt time.Time
     AuthorizedResources []any
 }

 type TokenCache struct {
     mu    sync.Mutex
     cache map[string]UserAuth
 }

 func NewTokenCache() *TokenCache {
     return &TokenCache{
         cache: make(map[string]UserAuth),
     }
 }


// handle the graphql queries for a single endpoint
type graphHandler struct {
	graph      string
	gqlHandler *handler.Handler
	timestamp  string
	client     gripql.Client
    tokenCache *TokenCache
	//schema     *gripql.Graph
}

// Handler is a GraphQL endpoint to query the Grip database
type Handler struct {
	handlers map[string]*graphHandler
	client   gripql.Client
}


func getAuthMappings(url string, token string) (any, error) {
     GetRequest, err := http.NewRequest("GET", url, nil)
     if err != nil {
         log.Error(err)
         return nil, err
     }

     GetRequest.Header.Set("Authorization", token)
     GetRequest.Header.Set("Accept", "application/json")
     fetchedData, err := http.DefaultClient.Do(GetRequest)
     if err != nil {
         log.Error(err)
         return nil, err
     }
     defer fetchedData.Body.Close()

     if fetchedData.StatusCode == http.StatusOK {
         bodyBytes, err := io.ReadAll(fetchedData.Body)
         if err != nil {
             log.Error(err)
         }

         var parsedData any
         err = json.Unmarshal(bodyBytes, &parsedData)
         if err != nil {
             log.Error(err)
             return nil, err
         }
         return parsedData, nil

     }
     // code must be nonNull to get here, probably don't want to cache a failed state
     empty_map :=  make(map[string]any)
     err = errors.New("Arborist auth/mapping GET returned a non-200 status code: " + fetchedData.Status)
     return empty_map, err
 }

 func hasPermission(permissions []any) bool {
     for _, permission := range permissions {
         permission := permission.(map[string]any)
         if (permission["service"] == "*" || permission["service"] == "peregrine") &&
             (permission["method"] == "*" || permission["method"] == "read") {
             // fmt.Println("PERMISSIONS: ", permission)
             return true
         }
     }
     return false
 }

 func getAllowedProjects(url string, token string) ([]any, error) {
     var readAccessResources []string
     authMappings, err := getAuthMappings(url, token)
     if err != nil {
         return nil, err
     }
 
     // Iterate through /auth/mapping resultant dict checking for valid read permissions
     for resourcePath, permissions := range authMappings.(map[string]any) {
         // fmt.Println("RESOURCE PATH: ", resourcePath)
 
         if hasPermission(permissions.([]any)) {
             readAccessResources = append(readAccessResources, resourcePath)
         }
     }
     // fmt.Println("VALUE OF READ ACCESS RESOURCES: ", readAccessResources)
 
     s := make([]interface{}, len(readAccessResources))
     for i, v := range readAccessResources {
         s[i] = v
     }
 
     // This readAccessResources value might need to be cached if the resources list gets too long
     return s, nil
 }

// NewClientHTTPHandler initilizes a new GraphQLHandler
func NewHTTPHandler(client gripql.Client) (http.Handler, error) {
	h := &Handler{
		client:   client,
		handlers: map[string]*graphHandler{},
	}
	return h, nil
}

// Static HTML that links to Apollo GraphQL query editor
var sandBox = `
<div id="sandbox" style="position:absolute;top:0;right:0;bottom:0;left:0"></div>
<script src="https://embeddable-sandbox.cdn.apollographql.com/_latest/embeddable-sandbox.umd.production.min.js"></script>
<script>
 new window.EmbeddedSandbox({
   target: "#sandbox",
   // Pass through your server href if you are embedding on an endpoint.
   // Otherwise, you can pass whatever endpoint you want Sandbox to start up with here.
   initialEndpoint: window.location.href,
 });
 // advanced options: https://www.apollographql.com/docs/studio/explorer/sandbox#embedding-sandbox
</script>`

// ServeHTTP responds to HTTP graphql requests
func (gh *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	//log.Infof("Request for %s", request.URL.Path)
	//If no graph provided, return the Query Editor page
	if request.URL.Path == "" || request.URL.Path == "/" {
		writer.Write([]byte(sandBox))
		return
	}
	//pathRE := regexp.MustCompile("/(.+)$")
	//graphName := pathRE.FindStringSubmatch(request.URL.Path)[1]
	graphName := request.URL.Path
	var handler *graphHandler
	var ok bool
	if handler, ok = gh.handlers[graphName]; ok {
		//Call the setup function. If nothing has changed it will return without doing anything
		err := handler.setup(request.Header)
		if err != nil {
			http.Error(writer, fmt.Sprintf("No GraphQL handler found for graph: %s", graphName), http.StatusInternalServerError)
			return
		}
	} else {

        tokenCache := NewTokenCache()
		//Graph handler was not found, so we'll need to set it up
		var err error
		handler, err = newGraphHandler(graphName, gh.client, request.Header, tokenCache)
		if err != nil {
			http.Error(writer, fmt.Sprintf("No GraphQL handler found for graph: %s", graphName), http.StatusInternalServerError)
			return
		}
		gh.handlers[graphName] = handler
	}
	if handler != nil && handler.gqlHandler != nil {
		handler.gqlHandler.ServeHTTP(writer, request)
	} else {
		http.Error(writer, fmt.Sprintf("No GraphQL handler found for graph: %s", graphName), http.StatusInternalServerError)
	}
}

// newGraphHandler creates a new graphql handler from schema
func newGraphHandler(graph string, client gripql.Client, headers http.Header, userCache *TokenCache) (*graphHandler, error) {
	o := &graphHandler{
		graph:  graph,
		client: client,
        tokenCache: userCache,
	}
	err := o.setup(headers)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// LookupToken looks up a user token in the cache based on the token string.
func (tc *TokenCache) LookupToken(token string) ([]any, bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	auth, tokenExists:= tc.cache[token]
    if auth.AuthorizedResources != nil{
        tokenExists = true
    }else{
        tokenExists = false
    }
    var resourceList []any
    if auth.AuthorizedResources != nil {
        resourceList = auth.AuthorizedResources
    }
    fmt.Println("RESOURCES LIST INSIDE LOOKUP TOKEN", resourceList)
	return resourceList, tokenExists
}

// JWT auth token storage function
func (tc *TokenCache) StoreToken(token string, auth UserAuth) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache[token] = auth
}

// Check timestamp to see if schema needs to be updated or if the access token has changed
// If so rebuild the schema
func (gh *graphHandler) setup(headers http.Header) error {
	ts, _ := gh.client.GetTimestamp(gh.graph)

    /*resourceList, ResourcesExist := gh.tokenCache.LookupToken(headers["Authorization"][0])

    // also chesk to see if token has expired
    if !ResourcesExist{
        resourceList, err := getAllowedProjects("http://arborist-service/auth/mapping",headers["Authorization"][0])
        if err != nil {
            log.WithFields(log.Fields{"graph": gh.graph, "error": err}).Error("auth/mapping fetch and processing step failed")
        }
        userAuth := UserAuth{
            AuthorizedResources: resourceList,
            ExpiresAt: time.Now().Add(time.Hour),
        }
        gh.tokenCache.StoreToken(headers["Authorization"][0], userAuth)
    }
    fmt.Println("ENTERING SETUP FUNCTION ++++++++++++++++++++++++ ____________________ ------------_________++++++++++++++++++++", resourceList)
    */

    
    resourceList, err := getAllowedProjects("http://arborist-service/auth/mapping",headers["Authorization"][0])
         if err != nil {
             log.WithFields(log.Fields{"graph": gh.graph, "error": err}).Error("auth/mapping fetch and processing step failed")
         }

	if ts == nil || ts.Timestamp != gh.timestamp || resourceList != nil {
        fmt.Println("YOU ARE HERE +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++", resourceList)
		log.WithFields(log.Fields{"graph": gh.graph}).Info("Reloading GraphQL schema")
		schema, err := gh.client.GetSchema(gh.graph)
		if err != nil {
			log.WithFields(log.Fields{"graph": gh.graph, "error": err}).Error("GetSchema error")
			return err
		}
		gqlSchema, err := buildGraphQLSchema(schema, gh.client, gh.graph, resourceList)
		if err != nil {
			log.WithFields(log.Fields{"graph": gh.graph, "error": err}).Error("GraphQL schema build failed")
			gh.gqlHandler = nil
			gh.timestamp = ""
		} else {
			log.WithFields(log.Fields{"graph": gh.graph}).Info("Built GraphQL schema")
			gh.gqlHandler = handler.New(&handler.Config{
				Schema: gqlSchema,
			})
			gh.timestamp = ts.Timestamp
		}
	}
	return nil
}
