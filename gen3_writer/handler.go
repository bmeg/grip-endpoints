/*
RESTFUL Gin Web endpoint
*/

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/bmeg/grip-graphql/middleware"
	"github.com/bmeg/grip/gdbi"
	"github.com/bmeg/grip/gripql"
	"github.com/bmeg/grip/log"
	"github.com/bmeg/grip/mongo"
	"github.com/bmeg/grip/util"
	"github.com/gin-gonic/gin"
	"github.com/mongodb/mongo-tools/common/db"
	"go.mongodb.org/mongo-driver/bson"
	mgo "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct {
	router *gin.Engine
	client gripql.Client
	config map[string]string
}

func NewHTTPHandler(client gripql.Client, config map[string]string) (http.Handler, error) {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Was getting 404s before adding this. Not 100% sure why
	r.RemoveExtraSlash = true

	r.NoRoute(func(c *gin.Context) {
		fmt.Printf("RAW PATH 404: %#v\n", c.Request.URL.RawPath)
		fmt.Printf("PATH 404: %#v\n", c.Request.URL.Path)

	})

	h := &Handler{
		router: r,
		client: client,
		config: config,
	}

	r.POST(":graph/add-vertex", func(c *gin.Context) {
		h.WriteVertex(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.POST(":graph/add-graph", func(c *gin.Context) {
		h.AddGraph(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.POST(":graph/bulk-load", func(c *gin.Context) {
		h.MongoBulk(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.POST(":graph/bulk-test", func(c *gin.Context) {
		h.BulkStream(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.DELETE(":graph/del-graph", func(c *gin.Context) {
		h.DeleteGraph(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.DELETE(":graph/del-edge/:edge-id", func(c *gin.Context) {
		h.DeleteEdge(c, c.Writer, c.Request, c.Param("graph"), c.Param("edge-id"))
	})
	r.DELETE(":graph/del-vertex/:vertex-id", func(c *gin.Context) {
		h.DeleteVertex(c, c.Writer, c.Request, c.Param("graph"), c.Param("vertex-id"))
	})
	r.GET(":graph/list-labels", func(c *gin.Context) {
		h.ListLabels(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.GET(":graph/get-schema", func(c *gin.Context) {
		h.GetSchema(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.GET(":graph/get-graph", func(c *gin.Context) {
		h.GetGraph(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.GET(":graph/get-vertex/:vertex-id", func(c *gin.Context) {
		h.GetVertex(c, c.Writer, c.Request, c.Param("graph"), c.Param("vertex-id"))
	})

	r.GET("list-graphs", func(c *gin.Context) {
		fmt.Printf("RAW PATH: %#v\n", c.Request.URL.RawPath)
		fmt.Printf("PATH: %#v\n", c.Request.URL.Path)
		h.ListGraphs(c, c.Writer)
	})

	return h, nil
}

// ServeHTTP responds to HTTP graphql requests
func (gh *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	gh.router.ServeHTTP(writer, request)
}

func RegError(c *gin.Context, writer http.ResponseWriter, graph string, err error) {
	log.WithFields(log.Fields{"graph": graph, "error": err})
	if ae, ok := err.(*middleware.ServerError); ok {
		c.JSON(ae.StatusCode, gin.H{
			"status":  ae.StatusCode,
			"message": ae.Message,
			"data":    nil,
		})
		http.Error(writer, fmt.Sprintln("[", ae.StatusCode, "] graph: ", graph, "error: ", ae.Message), ae.StatusCode)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  "500",
		"message": "Internal Server Error",
		"data":    nil,
	})
	http.Error(writer, fmt.Sprintln("[500]	graph", graph, "error:", err), http.StatusInternalServerError)
}

func (gh *Handler) ListLabels(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	if labels, err := gh.client.ListLabels(graph); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info(labels)
		http.Error(writer, fmt.Sprintln("[200]	GET:", graph, " ", labels), http.StatusOK)
	}
}

func (gh *Handler) GetSchema(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	if schema, err := gh.client.GetSchema(graph); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info(schema)
		writer.WriteHeader(http.StatusOK)
		fmt.Fprintln(writer, fmt.Sprintln("[200]  GET:", graph, " ", schema))
	}
}

func (gh *Handler) GetGraph(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	if graph_data, err := gh.client.GetMapping(graph); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info(graph_data)
		http.Error(writer, fmt.Sprintln("[200]	GET:", graph, " ", graph_data), http.StatusOK)
	}
}

func (gh *Handler) ListGraphs(c *gin.Context, writer http.ResponseWriter) {
	if graphs, err := gh.client.ListGraphs(); err != nil {
		RegError(c, writer, "", err)
	} else if err == nil {
		log.WithFields(log.Fields{}).Info(graphs)
		c.JSON(http.StatusOK, gin.H{
			"status":  "200",
			"message": "GET list-graphs successful",
			"data":    graphs,
		})
	}
}

func (gh *Handler) AddGraph(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	if err := gh.client.AddGraph(graph); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{}).Info("[200]	POST:", graph)
		http.Error(writer, fmt.Sprintln("[200]	POST:", graph), http.StatusOK)
	}
}

func (gh *Handler) DeleteGraph(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	if err := gh.client.DeleteGraph(graph); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{}).Info("[200]	DELETE:", graph)
		http.Error(writer, fmt.Sprintln("[200]	DELETE:", graph), http.StatusOK)
	}
}

func (gh *Handler) GetVertex(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string, vertex string) {
	if vertex, err := gh.client.GetVertex(graph, vertex); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info(vertex)
		http.Error(writer, fmt.Sprintln("[200]	GET:", graph, "VERTEX:", vertex), http.StatusOK)
	}
}

func (gh *Handler) GetEdge(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string, edge string) {
	if edge, err := gh.client.GetEdge(graph, edge); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info(edge)
		http.Error(writer, fmt.Sprintln("[200]	GET:", graph, "EDGE:", edge), http.StatusOK)
	}
}

func (gh *Handler) DeleteEdge(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string, edge string) {
	if _, err := gh.client.GetEdge(graph, edge); err == nil {
		if err := gh.client.DeleteEdge(graph, edge); err != nil {
			RegError(c, writer, graph, err)
		} else {
			log.WithFields(log.Fields{"graph": graph}).Info(edge)
			http.Error(writer, fmt.Sprintln("[200]	DELETE:", graph, "EDGE:", edge), http.StatusOK)
		}
	} else {
		RegError(c, writer, graph, err)
	}
}

func (gh *Handler) DeleteVertex(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string, vertex string) {
	if _, err := gh.client.GetVertex(graph, vertex); err == nil {
		if err := gh.client.DeleteVertex(graph, vertex); err != nil {
			RegError(c, writer, graph, err)
		} else {
			log.WithFields(log.Fields{"graph": graph}).Info(vertex)
			http.Error(writer, fmt.Sprintln("[200]	DELETE:", graph, "VERTEX:", vertex), http.StatusOK)
		}
	} else {
		RegError(c, writer, graph, err)
	}
}

func (gh *Handler) WriteVertex(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	var body []byte
	var err error
	v := &gripql.Vertex{}

	if body, err = io.ReadAll(request.Body); err != nil {
		RegError(c, writer, graph, err)
		return
	}
	if body == nil {
		RegError(c, writer, graph, err)
		return
	} else {
		if err := protojson.Unmarshal([]byte(body), v); err != nil {
			RegError(c, writer, graph, err)
			return
		}
	}
	if err := gh.client.AddVertex(graph, v); err != nil {
		RegError(c, writer, graph, err)
	} else {
		log.WithFields(log.Fields{"graph": graph}).Info("[200]	POST	VERTEX: ", v)
		http.Error(writer, fmt.Sprintln("[200]	POST	VERTEX: ", v), http.StatusOK)
	}
}

func HandleBody(request *http.Request) (map[string]any, error) {
	var body []byte
	var err error
	json_map := map[string]any{}

	if body, err = io.ReadAll(request.Body); err != nil {
		return nil, err
	}

	if body == nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(body), &json_map); err != nil {
		return nil, err
	}

	return json_map, nil
}

func (gh *Handler) MongoBulk(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) {
	var workerCount = 50
	var database = "gripdb"
	var logRate = 10000
	var bulkBufferSize = 1000

	err := request.ParseMultipartForm(1024 * 1024 * 1024) // 10 GB limit
	if err != nil {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 400, Message: fmt.Sprintf("failed to parse multipart form: %s", err)})
		return
	}

	args := request.MultipartForm.Value
	request_type_list, ok := args["type"]
	if !ok {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 400, Message: "Server must specify GraphElement Type, no value found for 'type'"})
		return
	}
	request_type := request_type_list[0]
	fill_gid_list, ok := args["fill_gid"]
	var fill_gid string
	if ok {
		fill_gid = fill_gid_list[0]
	}

	mongoHost_list, ok := args["mongo_host"]
	var mongoHost string
	if ok {
		mongoHost = mongoHost_list[0]
	} else if !ok {
		mongoHost = "mongodb://local-mongodb"
	}

	file, handler, err := request.FormFile("file")
	if err != nil {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 400, Message: fmt.Sprintf("failed to parse attached file: %s", err)})
		return
	}
	file.Close()

	client, err := mgo.NewClient(options.Client().ApplyURI(mongoHost))
	if err != nil {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 500, Message: fmt.Sprintf("%s", err)})
		return
	}

	err = client.Connect(context.TODO())
	if err != nil {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 500, Message: fmt.Sprintf("%s", err)})
		return
	}

	vertexCol := client.Database(database).Collection(fmt.Sprintf("%s_vertices", graph))
	edgeCol := client.Database(database).Collection(fmt.Sprintf("%s_edges", graph))

	if request_type == "vertex" {
		log.Infof("Loading vertex file: %s", handler.Filename)
		vertInserter := db.NewUnorderedBufferedBulkInserter(vertexCol, bulkBufferSize).
			SetBypassDocumentValidation(true).
			SetOrdered(false).
			SetUpsert(true)

		vertChan, err := StreamVerticesFromReader(file, workerCount)
		if err != nil {
			RegError(c, writer, graph, &middleware.ServerError{StatusCode: 500, Message: fmt.Sprintf("%s", err)})
			return
		}
		dataChan := vertexSerialize(vertChan, workerCount)
		count := 0
		for d := range dataChan {
			vertInserter.InsertRaw(d)
			if count%logRate == 0 {
				log.Infof("Loaded %d vertices", count)
			}
			count++
		}
		log.Infof("Loaded %d vertices", count)
		vertInserter.Flush()
	}
	if request_type == "edge" {
		log.Infof("Loading edge file: %s", handler.Filename)
		edgeInserter := db.NewUnorderedBufferedBulkInserter(edgeCol, bulkBufferSize).
			SetBypassDocumentValidation(true).
			SetOrdered(false).
			SetUpsert(true)

		edgeChan, err := StreamEdgesFromReader(file, workerCount)
		if err != nil {
			RegError(c, writer, graph, &middleware.ServerError{StatusCode: 500, Message: fmt.Sprintf("%s", err)})
			return
		}
		dataChan := edgeSerialize(edgeChan, fill_gid, workerCount)
		count := 0
		for d := range dataChan {
			edgeInserter.InsertRaw(d)
			if count%logRate == 0 {
				log.Infof("Loaded %d edges", count)
			}
			count++
		}
		log.Infof("Loaded %d vertices", count)
		edgeInserter.Flush()
	}

	responseData := map[string]string{"status": "200", "message": "File uploaded successfully"}
	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		RegError(c, writer, graph, &middleware.ServerError{StatusCode: 500, Message: fmt.Sprintf("%s", err)})
		return
	}

	writer.WriteHeader(http.StatusOK)
	writer.Write(responseJSON)
}

func vertexSerialize(vertChan chan *gripql.Vertex, workers int) chan []byte {
	dataChan := make(chan []byte, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for v := range vertChan {
				doc := mongo.PackVertex(gdbi.NewElementFromVertex(v))
				rawBytes, err := bson.Marshal(doc)
				if err == nil {
					dataChan <- rawBytes
				}
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(dataChan)
	}()
	return dataChan
}

func edgeSerialize(edgeChan chan *gripql.Edge, fill_gid string, workers int) chan []byte {
	dataChan := make(chan []byte, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for e := range edgeChan {
				if fill_gid != "" && e.Gid == "" {
					e.Gid = util.UUID()
				}
				doc := mongo.PackEdge(gdbi.NewElementFromEdge(e))
				rawBytes, err := bson.Marshal(doc)
				if err == nil {
					dataChan <- rawBytes
				}
			}
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(dataChan)
	}()
	return dataChan
}

func (gh *Handler) BulkStream(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) error {
	err := request.ParseMultipartForm(1024 * 1024 * 1024) // 10 GB limit
	if err != nil {
		fmt.Println("ERROR: ", err)
		http.Error(writer, "Error parsing form", http.StatusBadRequest)
		return err
	}

	request_type := request.MultipartForm.Value["type"][0]
	fmt.Println("VALUE OF REQUEST TYPE: ", request_type)

	// Get the file from the form data
	file, handler, err := request.FormFile("file")
	if err != nil {
		http.Error(writer, "Error retrieving file from form", http.StatusBadRequest)
		return err
	}

	defer file.Close()
	fmt.Println("FILE RECIEVED: ", handler.Filename, "REQUEST TYPE: ", request_type)
	var logRate = 10000

	if request_type == "vertex" {
		VertChan, err := StreamVerticesFromReader(file, 5)
		if err != nil {
			return err
		}
		count := 0
		elemChan := make(chan *gripql.GraphElement)
		go func() {
			if err := gh.client.BulkAdd(elemChan); err != nil {
				log.Errorf("bulk add error: %v", err)
			}
		}()
		for v := range VertChan {
			count++
			if count%logRate == 0 {
				log.Infof("Loaded %d vertices", count)
			}
			elemChan <- &gripql.GraphElement{Graph: graph, Vertex: v}
		}
		close(elemChan)
		log.Infof("Loaded total of %d vertices", count)
	}

	/*
	   if request_type == "edge" {
	       EdgeChan, err := StreamEdgesFromReader(file, 5)
	       if err != nil{
	           return err
	       }
	       count := 0
	       for e := range EdgeChan {
	           count++
	           if count % logRate == 0 {
	               log.Infof("Loaded %d vertices", count)
	           }
	           elemChan <- &gripql.GraphElement{Graph: graph, Edge: e}
	       }
	       log.Infof("Loaded total of %d edges", count)
	   }*/

	//<-wait

	responseData := map[string]string{"status": "200", "message": "File uploaded successfully"}
	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		http.Error(writer, "Error encoding JSON response", http.StatusInternalServerError)
		return err
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write(responseJSON)
	return nil

}

func StreamEdgesFromReader(reader io.Reader, workers int) (chan *gripql.Edge, error) {
	if workers < 1 {
		workers = 1
	}
	if workers > 99 {
		workers = 99
	}
	lineChan, err := processReader(reader, workers)
	if err != nil {
		return nil, err
	}

	edgeChan := make(chan *gripql.Edge, workers)
	var wg sync.WaitGroup

	jum := protojson.UnmarshalOptions{DiscardUnknown: true}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for line := range lineChan {
				e := &gripql.Edge{}
				err := jum.Unmarshal([]byte(line), e)
				if err != nil {
					log.WithFields(log.Fields{"error": err}).Errorf("Unmarshaling edge: %s", line)

				} else {
					edgeChan <- e
				}
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(edgeChan)
	}()

	return edgeChan, nil
}
func StreamVerticesFromReader(reader io.Reader, workers int) (chan *gripql.Vertex, error) {
	if workers < 1 {
		workers = 1
	}
	if workers > 99 {
		workers = 99
	}
	lineChan, err := processReader(reader, workers)
	if err != nil {
		return nil, err
	}

	vertChan := make(chan *gripql.Vertex, workers)
	var wg sync.WaitGroup

	jum := protojson.UnmarshalOptions{DiscardUnknown: true}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for line := range lineChan {
				v := &gripql.Vertex{}
				err := jum.Unmarshal([]byte(line), v)
				if err != nil {
					log.WithFields(log.Fields{"error": err}).Errorf("Unmarshaling vertex: %s", line)
				} else {
					vertChan <- v
				}
			}
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(vertChan)
	}()

	return vertChan, nil
}

func processReader(reader io.Reader, chansize int) (<-chan string, error) {
	scanner := bufio.NewScanner(reader)

	buf := make([]byte, 0, 64*1024)
	maxCapacity := 16 * 1024 * 1024
	scanner.Buffer(buf, maxCapacity)

	lineChan := make(chan string, chansize)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			lineChan <- line
		}

		if err := scanner.Err(); err != nil {
			log.WithFields(log.Fields{"error reading from reader: ": err})
		}
		close(lineChan)
	}()

	return lineChan, nil
}
