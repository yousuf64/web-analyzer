package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"shared/messagebus"
	"slices"
	"sync"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSConnection struct {
	conn   *websocket.Conn
	jobIDs []string
	mu     sync.RWMutex
}

var (
	wsConnections = make(map[*WSConnection]bool)
	wsLock        sync.RWMutex
)

func (wsc *WSConnection) addJobID(jobID string) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	if !slices.Contains(wsc.jobIDs, jobID) {
		wsc.jobIDs = append(wsc.jobIDs, jobID)
	}
}

func (wsc *WSConnection) hasJobID(jobID string) bool {
	wsc.mu.RLock()
	defer wsc.mu.RUnlock()
	return slices.Contains(wsc.jobIDs, jobID)
}

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
		// Broadcast to all connections if no specific job ID
		msgJobID = ""
	default:
		// Broadcast to all connections if no specific job ID
		msgJobID = ""
	}

	for conn := range wsConnections {
		// If message has a specific job ID, only send to connections subscribed to that job
		if msgJobID != "" && !conn.hasJobID(msgJobID) {
			continue
		}

		err := conn.conn.WriteMessage(websocket.TextMessage, jsonMessage)
		if err != nil {
			log.Printf("Failed to write to websocket: %v", err)
			// Remove connection on error
			go func(c *WSConnection) {
				wsLock.Lock()
				delete(wsConnections, c)
				wsLock.Unlock()
				c.conn.Close()
			}(conn)
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
		log.Printf("Broadcasting job update: %+v", m)
		broadcastToUsers(m)
	})

	mb.SubscribeToTaskStatusUpdate(func(msg *nats.Msg) {
		var m messagebus.TaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("Failed to unmarshal task update: %v", err)
			return
		}
		log.Printf("Broadcasting task status update: %+v", m)
		broadcastToUsers(m)
	})

	mb.SubscribeToSubTaskStatusUpdate(func(msg *nats.Msg) {
		var m messagebus.SubTaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("Failed to unmarshal subtask update: %v", err)
			return
		}
		log.Printf("Broadcasting subtask status update: %+v", m)
		broadcastToUsers(m)
	})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket connection: %v", err)
		return
	}
	defer conn.Close()

	wsConn := &WSConnection{
		conn:   conn,
		jobIDs: []string{},
	}

	// Add the connection to the map
	wsLock.Lock()
	wsConnections[wsConn] = true
	wsLock.Unlock()

	log.Println("New WebSocket connection established")

	// Remove the connection from the map on return
	defer func() {
		wsLock.Lock()
		delete(wsConnections, wsConn)
		wsLock.Unlock()
		log.Println("WebSocket connection closed")
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected websocket close error: %v", err)
			}
			break
		}

		// Handle subscription updates from client
		if messageType == websocket.TextMessage {
			var subscriptionUpdate struct {
				Action string `json:"action"`
				JobID  string `json:"job_id"`
			}

			if err := json.Unmarshal(p, &subscriptionUpdate); err == nil {
				switch subscriptionUpdate.Action {
				case "subscribe":
					wsConn.addJobID(subscriptionUpdate.JobID)
					log.Printf("Added subscription for job ID: %s", subscriptionUpdate.JobID)
				case "unsubscribe":
					wsConn.mu.Lock()
					for i, id := range wsConn.jobIDs {
						if id == subscriptionUpdate.JobID {
							wsConn.jobIDs = append(wsConn.jobIDs[:i], wsConn.jobIDs[i+1:]...)
							break
						}
					}
					wsConn.mu.Unlock()
					log.Printf("Removed subscription for job ID: %s", subscriptionUpdate.JobID)
				}
			}
		}
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	setupSubscriptions(nc)

	http.HandleFunc("/ws", corsMiddleware(handleWebSocket))

	go func() {
		log.Printf("Notification backplane listening on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
	}()

	log.Println("Notification backplane service is running...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down notification backplane...")
}
