package notifications

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"shared/metrics"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Hub manages WebSocket connections and message broadcasting
type Hub struct {
	connections map[*Connection]bool
	mu          sync.RWMutex
	metrics     *metrics.NotificationsMetrics
	log         *slog.Logger
}

// HubOption configures the Hub
type HubOption func(*Hub)

// NewHub creates a new WebSocket hub with optional configurations
func NewHub(opts ...HubOption) *Hub {
	h := &Hub{
		connections: make(map[*Connection]bool),
		log:         slog.Default(),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// WithHubMetrics sets the metrics collector for the hub
func WithHubMetrics(m *metrics.NotificationsMetrics) HubOption {
	return func(h *Hub) { h.metrics = m }
}

// WithHubLogger sets the logger for the hub
func WithHubLogger(log *slog.Logger) HubOption {
	return func(h *Hub) { h.log = log }
}

// AddConnection adds a new WebSocket connection to the hub
func (h *Hub) AddConnection(conn *Connection) {
	h.mu.Lock()
	h.connections[conn] = true
	count := len(h.connections)
	h.mu.Unlock()

	if h.metrics != nil {
		h.metrics.RecordWebSocketConnection(true)
		h.metrics.SetActiveWebSocketConnections(count)
	}

	h.log.Info("New WebSocket connection established", slog.Int("total", count))
}

// RemoveConnection removes a WebSocket connection from the hub
func (h *Hub) RemoveConnection(conn *Connection) {
	h.mu.Lock()
	delete(h.connections, conn)
	count := len(h.connections)
	h.mu.Unlock()

	if h.metrics != nil {
		d := time.Since(conn.start).Seconds()
		h.metrics.RecordWebSocketConnectionDuration(d)
		h.metrics.SetActiveWebSocketConnections(count)
	}

	h.log.Info("WebSocket connection closed", slog.Int("total", count))
}

// BroadcastToGroup sends a message to all connections subscribed to a specific group
func (h *Hub) BroadcastToGroup(msg any, group string) {
	start := time.Now()

	data, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("Failed to marshal message", slog.Any("error", err))
		return
	}

	msgType := h.extractMessageType(msg)

	h.mu.RLock()
	defer h.mu.RUnlock()

	successCount := 0
	totalCount := 0

	for conn := range h.connections {
		// If group specified, only send to connections subscribed to that group
		if group != "" && !conn.HasGroup(group) {
			continue
		}

		totalCount++
		if err := conn.WriteMessage(data); err != nil {
			h.log.Error("Failed to write to websocket", slog.Any("error", err))
			if h.metrics != nil {
				h.metrics.RecordWebSocketMessage(msgType, false, 0)
			}

			// Remove connection on error
			go func(c *Connection) {
				h.RemoveConnection(c)
				c.Close()
			}(conn)
		} else {
			successCount++
		}
	}

	if totalCount > 0 && h.metrics != nil {
		d := time.Since(start).Seconds()
		h.metrics.RecordWebSocketMessage(msgType, successCount == totalCount, d)
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msg any) {
	h.BroadcastToGroup(msg, "")
}

// RecordGroupSubscription records subscription metrics
func (h *Hub) RecordGroupSubscription(action, group string) {
	if h.metrics != nil {
		h.metrics.RecordGroupSubscription(action, group)
	}
}

// Close shuts down the hub and closes all connections
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.connections {
		conn.Close()
	}

	h.connections = make(map[*Connection]bool)
	h.log.Info("WebSocket hub closed")
}

// extractMessageType extracts the message type for metrics
func (h *Hub) extractMessageType(msg any) string {
	if msgMap, ok := msg.(map[string]interface{}); ok {
		if t, exists := msgMap["type"]; exists {
			if typeStr, ok := t.(string); ok {
				return typeStr
			}
		}
	}
	return "unknown"
}

// Connection represents a WebSocket connection with group subscriptions
type Connection struct {
	conn   *websocket.Conn
	groups []string
	mu     sync.RWMutex
	hub    *Hub
	log    *slog.Logger
	start  time.Time
}

// SubscriptionMessage represents a subscription/unsubscription request
type SubscriptionMessage struct {
	Action string `json:"action"`
	Group  string `json:"group"`
}

// NewConnection creates a new WebSocket connection wrapper
func NewConnection(conn *websocket.Conn, hub *Hub, log *slog.Logger) *Connection {
	return &Connection{
		conn:   conn,
		groups: make([]string, 0),
		hub:    hub,
		log:    log,
		start:  time.Now(),
	}
}

// AddGroup adds the connection to a subscription group
func (c *Connection) AddGroup(group string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !slices.Contains(c.groups, group) {
		c.groups = append(c.groups, group)
	}
}

// RemoveGroup removes the connection from a subscription group
func (c *Connection) RemoveGroup(group string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, g := range c.groups {
		if g == group {
			c.groups = append(c.groups[:i], c.groups[i+1:]...)
			break
		}
	}
}

// HasGroup checks if the connection is subscribed to a group
func (c *Connection) HasGroup(group string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Contains(c.groups, group)
}

// WriteMessage sends a message to the WebSocket connection
func (c *Connection) WriteMessage(msg []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

// Close closes the WebSocket connection
func (c *Connection) Close() error {
	return c.conn.Close()
}

// ReadLoop continuously reads messages from the WebSocket connection
func (c *Connection) ReadLoop() {
	defer func() {
		c.hub.RemoveConnection(c)
		c.conn.Close()
	}()

	for {
		msgType, p, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error("Unexpected websocket close error", slog.Any("error", err))
			}
			break
		}

		if msgType == websocket.TextMessage {
			c.handleSubscriptionMessage(p)
		}
	}
}

// handleSubscriptionMessage processes subscription/unsubscription requests
func (c *Connection) handleSubscriptionMessage(data []byte) {
	var sub SubscriptionMessage
	if err := json.Unmarshal(data, &sub); err != nil {
		c.log.Error("Failed to unmarshal subscription message", slog.Any("error", err))
		return
	}

	switch sub.Action {
	case "subscribe":
		c.AddGroup(sub.Group)
		c.hub.RecordGroupSubscription("subscribe", sub.Group)
		c.log.Info("Added subscription for group", slog.String("group", sub.Group))

	case "unsubscribe":
		c.RemoveGroup(sub.Group)
		c.hub.RecordGroupSubscription("unsubscribe", sub.Group)
		c.log.Info("Removed subscription for group", slog.String("group", sub.Group))
	}
}

// Handler handles WebSocket HTTP requests and upgrades them to WebSocket connections
type Handler struct {
	hub *Hub
	log *slog.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(hub *Hub, log *slog.Logger) *Handler {
	return &Handler{
		hub: hub,
		log: log,
	}
}

// HandleWebSocket upgrades HTTP requests to WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("Failed to upgrade websocket connection", slog.Any("error", err))
		return
	}

	// Create connection wrapper
	wsConn := NewConnection(conn, h.hub, h.log)

	// Add to hub
	h.hub.AddConnection(wsConn)

	// Start reading messages in goroutine
	go wsConn.ReadLoop()
}
