package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	notificationsServiceName = "notifications"
)

// NotificationsMetrics is a struct for notifications metrics
type NotificationsMetrics struct {
	*ServiceMetrics

	WebSocketConnectionsActive        prometheus.Gauge
	WebSocketConnectionsTotal         *prometheus.CounterVec
	WebSocketMessagesSentTotal        *prometheus.CounterVec
	WebSocketMessageBroadcastDuration *prometheus.HistogramVec
	WebSocketConnectionDuration       *prometheus.HistogramVec

	WebSocketSubscriptionsTotal  *prometheus.CounterVec
	WebSocketSubscriptionsActive *prometheus.GaugeVec
}

// NewNotificationsMetrics creates a new notifications metrics
func NewNotificationsMetrics() *NotificationsMetrics {
	baseMetrics := NewServiceMetrics(notificationsServiceName)

	notificationsMetrics := &NotificationsMetrics{
		ServiceMetrics: baseMetrics,

		WebSocketConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "websocket_connections_active",
				Help:        "Current number of active WebSocket connections",
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
		),

		WebSocketConnectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "websocket_connections_total",
				Help:        "Total number of WebSocket connections established",
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{LabelStatus},
		),

		WebSocketMessagesSentTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "websocket_messages_sent_total",
				Help:        "Total number of WebSocket messages sent",
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{LabelMessageType, LabelStatus},
		),

		WebSocketMessageBroadcastDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "websocket_message_broadcast_duration_seconds",
				Help:        "WebSocket message broadcast duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{LabelMessageType},
		),

		WebSocketConnectionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "websocket_connection_duration_seconds",
				Help:        "WebSocket connection duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{},
		),

		WebSocketSubscriptionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "websocket_group_subscriptions_total",
				Help:        "Total number of WebSocket group subscription events",
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{"action", "group"},
		),

		WebSocketSubscriptionsActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:        "websocket_group_subscriptions_active",
				Help:        "Current number of active WebSocket group subscriptions",
				ConstLabels: prometheus.Labels{LabelService: notificationsServiceName},
			},
			[]string{"group"},
		),
	}

	return notificationsMetrics
}

// MustRegisterNotifications registers the notifications metrics and base service metrics
func (m *NotificationsMetrics) MustRegisterNotifications() {
	m.ServiceMetrics.MustRegister()

	prometheus.MustRegister(
		m.WebSocketConnectionsActive,
		m.WebSocketConnectionsTotal,
		m.WebSocketMessagesSentTotal,
		m.WebSocketMessageBroadcastDuration,
		m.WebSocketConnectionDuration,
		m.WebSocketSubscriptionsTotal,
		m.WebSocketSubscriptionsActive,
	)
}

// RecordWebSocketConnection records the metrics for WebSocket connections
func (m *NotificationsMetrics) RecordWebSocketConnection(success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	m.WebSocketConnectionsTotal.WithLabelValues(status).Inc()
}

// SetActiveWebSocketConnections sets the active WebSocket connections metrics
func (m *NotificationsMetrics) SetActiveWebSocketConnections(count int) {
	m.WebSocketConnectionsActive.Set(float64(count))
}

// RecordWebSocketMessage records the metrics for WebSocket messages
func (m *NotificationsMetrics) RecordWebSocketMessage(messageType string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	m.WebSocketMessagesSentTotal.WithLabelValues(messageType, status).Inc()
	m.WebSocketMessageBroadcastDuration.WithLabelValues(messageType).Observe(duration)
}

// RecordWebSocketConnectionDuration records the metrics for WebSocket connection duration
func (m *NotificationsMetrics) RecordWebSocketConnectionDuration(duration float64) {
	m.WebSocketConnectionDuration.WithLabelValues().Observe(duration)
}

// RecordGroupSubscription records the metrics for WebSocket group subscriptions
func (m *NotificationsMetrics) RecordGroupSubscription(action, group string) {
	m.WebSocketSubscriptionsTotal.WithLabelValues(action, group).Inc()
}

// SetActiveGroupSubscriptions sets the active WebSocket group subscriptions metrics
func (m *NotificationsMetrics) SetActiveGroupSubscriptions(group string, count float64) {
	m.WebSocketSubscriptionsActive.WithLabelValues(group).Set(count)
}
