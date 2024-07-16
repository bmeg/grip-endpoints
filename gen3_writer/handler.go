/*
RESTFUL Gin Web endpoint
*/

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/bmeg/grip/gripql"
	"github.com/bmeg/grip/log"
	"github.com/bmeg/grip/util"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct {
	router *gin.Engine
	client gripql.Client
}

func NewHTTPHandler(client gripql.Client, config map[string]string) (http.Handler, error) {
	r := gin.Default()
	h := &Handler{
		router: r,
		client: client,
	}

	r.POST(":graph/add-vertex", func(c *gin.Context) {
		h.WriteVertex(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.POST(":graph/add-graph", func(c *gin.Context) {
		h.AddGraph(c, c.Writer, c.Request, c.Param("graph"))
	})
	r.POST(":graph/bulk-load", func(c *gin.Context) {
		h.BulkStream(c, c.Writer, c.Request, c.Param("graph"))
		//h.BulkLoad(c.Writer, c.Request, c.Param("graph"))
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
	r.GET(":graph", func(c *gin.Context) {
		if c.Param("graph") == "list-graphs" {
			h.ListGraphs(c, c.Writer)
		}
	})
	return h, nil
}

// ServeHTTP responds to HTTP graphql requests
func (gh *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	/*fmt.Println("REQUEST", request)
	fmt.Println("WRITER", writer)*/
	gh.router.ServeHTTP(writer, request)
}

func RegError(c *gin.Context, writer http.ResponseWriter, graph string, err error) {
	log.WithFields(log.Fields{"graph": graph, "error": err})
    c.JSON(http.StatusOK, gin.H{
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
		http.Error(writer, fmt.Sprintln("[200]	GET:", graph, " ", schema), http.StatusOK)
	}
}

// not sure what this does might want to delete. Maybe don't need the schema functions in here and do that manually
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
		RegError(c, writer,  "", err)
	} else if err == nil{
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


func HandleBody(request *http.Request) (map[string]any, error){
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
	fmt.Println("FILE RECIEVED: ", handler.Filename)
    var logRate = 10000

    elemChan := make(chan *gripql.GraphElement)
    go func() {
        if err := gh.client.BulkAdd(elemChan); err != nil {
            log.Errorf("bulk add error: %v", err)
        }
    }()

    if request_type == "vertex" {
        VertChan, err := StreamVerticesFromReader(file, 99)
        if err != nil{
            return err
        }
        count := 0

        for v := range VertChan {
            count++
            if count%logRate == 0 {
                log.Infof("Loaded %d vertices", count)
            }
            elemChan <- &gripql.GraphElement{Graph: graph, Vertex: v}
        }
        log.Infof("Loaded total of %d vertices", count)
    }

    if request_type == "edge" {
        EdgeChan, err := StreamEdgesFromReader(file, 99)
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
    }

    close(elemChan)
    //<-wait

    responseData := map[string]string{"status" : "200", "message": "File uploaded successfully"}
	responseJSON, err := json.Marshal(responseData)
	if err != nil {
		http.Error(writer, "Error encoding JSON response", http.StatusInternalServerError)
		return err
	}
	writer.WriteHeader(http.StatusOK)
	writer.Write(responseJSON)
	return nil

}

func StreamVerticesFromReader(reader io.Reader, workers int) (chan *gripql.Vertex, error) {
	if workers < 1 {
		workers = 1
	}
	if workers > 99 {
		workers = 99
	}

    lineChan, err := processReader(reader)
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

func StreamEdgesFromReader(reader io.Reader, workers int) (chan *gripql.Edge, error) {
     if workers < 1 {
         workers = 1
     }
     if workers > 99 {
         workers = 99
     }

     lineChan, err := processReader(reader)
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
                 v := &gripql.Edge{}
                 err := jum.Unmarshal([]byte(line), v)
                 if err != nil {
                     log.WithFields(log.Fields{"error": err}).Errorf("Unmarshaling edge: %s", line)
                 } else {
                     edgeChan <- v
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

func processReader(reader io.Reader) (<-chan string, error) {
	scanner := bufio.NewScanner(reader)

	chanSize := 100
	buf := make([]byte, 0, 64*1024)
	maxCapacity := 16 * 1024 * 1024
	scanner.Buffer(buf, maxCapacity)

	lineChan := make(chan string, chanSize)

	go func() {
        for scanner.Scan() {
			line := scanner.Text()
			lineChan <- line
		}

		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading from reader: %s", err)
		}
		close(lineChan)
	}()

	return lineChan, nil
}


func (gh *Handler) BulkLoad(c *gin.Context, writer http.ResponseWriter, request *http.Request, graph string) error {
	var workerCount = 1
	var logRate = 10000
    var err error;
    var json_map map[string]any;
	log.WithFields(log.Fields{"graph": graph}).Info("loading data")


    // Get request body to check for edges or vertices
    if json_map, err = HandleBody(request); err != nil{
        RegError(c, writer, graph, err)
    }

	elemChan := make(chan *gripql.GraphElement)
	go func() {
		if err := gh.client.BulkAdd(elemChan); err != nil {
			log.Errorf("bulk add error: %v", err)
		}
	}()

	// vertices and edges are expected to be ndjson format
	_, ok := json_map["vertex"]
	if ok {
		vertexFile := json_map["vertex"].(string)
		log.Infof("Loading vertex file: %s", vertexFile)
		count := 0
		vertChan, err := util.StreamVerticesFromFile(vertexFile, workerCount)
		if err != nil {
			log.Infof("ERROR: ", err)
			return err
		}
		for v := range vertChan {
			count++
			if count%logRate == 0 {
				log.Infof("Loaded %d vertices", count)
			}
			elemChan <- &gripql.GraphElement{Graph: graph, Vertex: v}
		}
		log.Infof("Loaded total of %d vertices", count)
	}

	_, ok = json_map["edge"]
	if ok {
		edgeFile := json_map["edge"].(string)
		log.Infof("Loading edge file: %s", edgeFile)
		count := 0
		edgeChan, err := util.StreamEdgesFromFile(edgeFile, workerCount)
		if err != nil {
			return err
		}
		for e := range edgeChan {
			count++
			if count%logRate == 0 {
				log.Infof("Loaded %d edges", count)
			}
			elemChan <- &gripql.GraphElement{Graph: graph, Edge: e}
		}
		log.Infof("Loaded total of %d edges", count)
	}

	close(elemChan)
	return nil
}
