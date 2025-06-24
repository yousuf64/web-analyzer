package main

import (
	"encoding/json"
	"log"
	"net/http"
	"shared/messagebus"
	"shared/repository"
	"shared/types"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
	"github.com/yousuf64/shift"
)

var jobRepo *repository.JobRepository
var taskRepo *repository.TaskRepository
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSConnection struct {
	conn   *websocket.Conn
	jobIDs []string
}

var (
	wsConnections = make(map[*WSConnection]bool)
	wsLock        sync.RWMutex
)

func broadcastToUsers(message any) {
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	wsLock.RLock()
	defer wsLock.RUnlock()

	var msgJobID string
	switch m := message.(type) {
	case messagebus.TaskStatusUpdateMessage:
		msgJobID = m.JobID
	case messagebus.SubTaskStatusUpdateMessage:
		msgJobID = m.JobID
	case messagebus.JobUpdateMessage:
		msgJobID = "" // Broadcast to all connections
	default:
		// Broadcast to all connections
	}

	for conn := range wsConnections {
		if msgJobID != "" && !slices.Contains(conn.jobIDs, msgJobID) {
			continue
		}

		err := conn.conn.WriteMessage(websocket.TextMessage, jsonMessage)
		if err != nil {
			log.Printf("Failed to write to websocket: %v", err)
		}
	}
}

func setupSubscriptions(nc *nats.Conn) {
	mb := messagebus.New(nc)
	mb.SubscribeToJobUpdate(func(msg *nats.Msg) {
		var m messagebus.JobUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("Failed to unmarshal job update: %v", err)
			return
		}
		broadcastToUsers(m)
	})

	mb.SubscribeToTaskStatusUpdate(func(msg *nats.Msg) {
		var m messagebus.TaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("Failed to unmarshal task update: %v", err)
			return
		}
		broadcastToUsers(m)
	})

	mb.SubscribeToSubTaskStatusUpdate(func(msg *nats.Msg) {
		var m messagebus.SubTaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("Failed to unmarshal subtask update: %v", err)
			return
		}
		broadcastToUsers(m)
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, route shift.Route) error {
	jobID := route.Params.Get("job_id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket connection: %v", err)
		return err
	}
	defer conn.Close()

	wsConn := &WSConnection{
		conn:   conn,
		jobIDs: []string{jobID},
	}

	// Add the connection to the map
	wsLock.Lock()
	wsConnections[wsConn] = true
	wsLock.Unlock()

	// Remove the connection from the map on return
	defer func() {
		wsLock.Lock()
		delete(wsConnections, wsConn)
		wsLock.Unlock()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected websocket close error: %v", err)
			}
			break
		}
	}

	return nil
}

func corsMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		return next(w, r, route)
	}
}

func errorMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		err := next(w, r, route)
		if err != nil {
			log.Printf("Error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}
}

func main() {
	dynamodb, err := repository.NewDynamoDBClient()
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client %v", err)
	}
	repository.SeedTables(dynamodb)

	jobRepo, err = repository.NewJobRepository()
	if err != nil {
		log.Fatalf("Failed to create job repo %v", err)
	}

	taskRepo, err = repository.NewTaskRepository()
	if err != nil {
		log.Fatalf("Failed to create task repo %v", err)
	}

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	setupSubscriptions(nc)

	router := shift.New()
	router.Use(corsMiddleware)
	router.Use(errorMiddleware)

	// Register OPTIONS handler for all routes, so that CORS is handled by the middleware
	router.OPTIONS("/*wildcard", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	router.POST("/analyze", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		var req types.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		jobId := strconv.Itoa(int(time.Now().UnixNano()))
		job := &types.Job{
			ID:        jobId,
			URL:       req.Url,
			Status:    types.JobStatusPending,
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		}
		err := jobRepo.CreateJob(job)

		if err != nil {
			return err
		}

		err = taskRepo.CreateTasks(&types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeExtracting,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeIdentifyingVersion,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeAnalyzing,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeVerifyingLinks,
			Status: types.TaskStatusPending,
		})
		if err != nil {
			return err
		}

		msg, err := json.Marshal(types.AnalyzeMessage{
			JobId: jobId,
		})
		if err != nil {
			return err
		}

		err = nc.Publish("url.analyze", msg)
		if err != nil {
			return err
		}
		log.Printf("Message published: %v", req)

		w.WriteHeader(http.StatusAccepted)
		return json.NewEncoder(w).Encode(types.AnalyzeResponse{
			Job: *job,
		})
	})

	router.GET("/jobs", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobs, err := jobRepo.GetAllJobs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(jobs)
	})

	router.GET("/jobs/:job_id/tasks", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobId := route.Params.Get("job_id")
		if jobId == "" {
			http.Error(w, "Job ID is required", http.StatusBadRequest)
			return nil
		}

		tasks, err := taskRepo.GetTasksByJobId(jobId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(tasks)
	})

	router.GET("/ws", handleWebSocket)
	router.GET("/ws/:job_id", handleWebSocket)

	log.Printf("API server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router.Serve()))
}
