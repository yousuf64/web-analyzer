package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"shared/log"
	"shared/messagebus"
	"shared/metrics"
	"shared/tracing"
	"slices"
	"sync"
	"syscall"
	"time"

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
	groups []string
	mu     sync.RWMutex
}

var (
	wsConnections = make(map[*WSConnection]bool)
	wsLock        sync.RWMutex
	subscriptions []*nats.Subscription
	logger        *slog.Logger
	mc            *metrics.NotificationsMetrics
)

func (wsc *WSConnection) addGroup(group string) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	if !slices.Contains(wsc.groups, group) {
		wsc.groups = append(wsc.groups, group)
	}
}

func (wsc *WSConnection) removeGroup(group string) {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()
	for i, g := range wsc.groups {
		if g == group {
			wsc.groups = append(wsc.groups[:i], wsc.groups[i+1:]...)
			break
		}
	}
}

func (wsc *WSConnection) hasGroup(group string) bool {
	wsc.mu.RLock()
	defer wsc.mu.RUnlock()
	return slices.Contains(wsc.groups, group)
}

func broadcastToUsers(message any, group string) {
	broadcastStart := time.Now()

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logger.Error("Failed to marshal message", slog.Any("error", err))
		return
	}

	wsLock.RLock()
	defer wsLock.RUnlock()

	messageType := "unknown"
	if msgMap, ok := message.(map[string]interface{}); ok {
		if t, exists := msgMap["type"]; exists {
			if typeStr, ok := t.(string); ok {
				messageType = typeStr
			}
		}
	}

	successCount := 0
	totalCount := 0

	for conn := range wsConnections {
		// If a group is specified, only send to connections subscribed to that group
		if group != "" && !conn.hasGroup(group) {
			continue
		}

		totalCount++
		err := conn.conn.WriteMessage(websocket.TextMessage, jsonMessage)
		if err != nil {
			logger.Error("Failed to write to websocket", slog.Any("error", err))
			mc.RecordWebSocketMessage(messageType, false, 0)

			// Remove connection on error
			go func(c *WSConnection) {
				wsLock.Lock()
				delete(wsConnections, c)
				mc.SetActiveWebSocketConnections(len(wsConnections))
				wsLock.Unlock()
				c.conn.Close()
			}(conn)
		} else {
			successCount++
		}
	}

	if totalCount > 0 {
		mc.RecordWebSocketMessage(messageType, successCount == totalCount, time.Since(broadcastStart).Seconds())
	}
}

func setupSubscriptions(nc *nats.Conn) {
	mb := messagebus.New(nc, mc)

	sub, err := mb.SubscribeToJobUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.JobUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			logger.Error("Failed to unmarshal job update", slog.Any("error", err))
			return
		}
		logger.Info("Broadcasting job update for job", slog.String("jobId", m.JobID))
		broadcastToUsers(m, "")
	})
	if err != nil {
		logger.Error("Failed to subscribe to job update", slog.Any("error", err))
		os.Exit(1)
	}
	subscriptions = append(subscriptions, sub)

	sub, err = mb.SubscribeToTaskStatusUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.TaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			logger.Error("Failed to unmarshal task update", slog.Any("error", err))
			return
		}

		logger.Info("Broadcasting task status update for job", slog.String("jobId", m.JobID))
		broadcastToUsers(m, m.JobID)
	})
	if err != nil {
		logger.Error("Failed to subscribe to task status update", slog.Any("error", err))
		os.Exit(1)
	}
	subscriptions = append(subscriptions, sub)

	sub, err = mb.SubscribeToSubTaskUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.SubTaskUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			logger.Error("Failed to unmarshal subtask update", slog.Any("error", err))
			return
		}

		logger.Info("Broadcasting subtask update for job",
			slog.String("jobId", m.JobID),
			slog.String("key", m.Key),
			slog.String("status", string(m.SubTask.Status)),
			slog.String("url", m.SubTask.URL),
			slog.String("description", m.SubTask.Description))
		broadcastToUsers(m, m.JobID)
	})
	if err != nil {
		logger.Error("Failed to subscribe to subtask update", slog.Any("error", err))
		os.Exit(1)
	}
	subscriptions = append(subscriptions, sub)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	connectionStart := time.Now()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade websocket connection", slog.Any("error", err))
		mc.RecordWebSocketConnection(false)
		return
	}
	defer conn.Close()

	wsConn := &WSConnection{
		conn:   conn,
		groups: []string{},
	}

	// Add the connection to the map
	wsLock.Lock()
	wsConnections[wsConn] = true
	connectionCount := len(wsConnections)
	wsLock.Unlock()

	// Record successful connection
	mc.RecordWebSocketConnection(true)
	mc.SetActiveWebSocketConnections(connectionCount)

	logger.Info("New WebSocket connection established")

	// Remove the connection from the map on return
	defer func() {
		connectionDuration := time.Since(connectionStart).Seconds()
		mc.RecordWebSocketConnectionDuration(connectionDuration)

		wsLock.Lock()
		delete(wsConnections, wsConn)
		connectionCount := len(wsConnections)
		wsLock.Unlock()

		mc.SetActiveWebSocketConnections(connectionCount)
		logger.Info("WebSocket connection closed")
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("Unexpected websocket close error", slog.Any("error", err))
			}
			break
		}

		// Handle subscription updates from client
		if messageType == websocket.TextMessage {
			var subscriptionUpdate struct {
				Action string `json:"action"`
				Group  string `json:"group"`
			}

			if err := json.Unmarshal(p, &subscriptionUpdate); err == nil {
				switch subscriptionUpdate.Action {
				case "subscribe":
					wsConn.addGroup(subscriptionUpdate.Group)
					mc.RecordGroupSubscription("subscribe", subscriptionUpdate.Group)
					logger.Info("Added subscription for group", slog.String("group", subscriptionUpdate.Group))

				case "unsubscribe":
					wsConn.removeGroup(subscriptionUpdate.Group)
					mc.RecordGroupSubscription("unsubscribe", subscriptionUpdate.Group)
					logger.Info("Removed subscription for group", slog.String("group", subscriptionUpdate.Group))

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
	logger = log.SetupFromEnv("notifications")
	logger.Info("Starting notifications service")

	ctx := context.Background()
	otelShutdown, err := tracing.SetupOTelSDK(ctx, "notifications")
	if err != nil {
		logger.Error("Failed to setup OTel SDK", slog.Any("error", err))
		os.Exit(1)
	}
	defer otelShutdown(ctx)

	// Initialize metrics
	mc = metrics.NewNotificationsMetrics()
	mc.MustRegisterNotifications()
	mc.SetServiceInfo("1.0.0", runtime.Version())

	// Start metrics server
	metricsServer := mc.StartMetricsServer("9092")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		metricsServer.Shutdown(ctx)
	}()

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", slog.Any("error", err))
		os.Exit(1)
	}
	defer nc.Close()

	setupSubscriptions(nc)

	http.HandleFunc("/ws", corsMiddleware(handleWebSocket))

	go func() {
		logger.Info("Notification backplane listening on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			logger.Error("Failed to listen", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	logger.Info("Notification backplane service is running...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Unsubscribing from NATS", slog.Int("subscriptionCount", len(subscriptions)))
	for _, sub := range subscriptions {
		sub.Unsubscribe()
	}

	logger.Info("Shutting down notification backplane...")
}
