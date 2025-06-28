package notifications

import (
	"context"
	"encoding/json"
	"log/slog"
	"notifications/internal/config"
	"shared/messagebus"

	"github.com/nats-io/nats.go"
)

// NotificationService handles WebSocket notifications and NATS message subscriptions
type NotificationService struct {
	hub  *Hub
	mb   *messagebus.MessageBus
	cfg  *config.Config
	log  *slog.Logger
	subs []*nats.Subscription
}

// Option configures the NotificationService
type Option func(*NotificationService)

// NewNotificationService creates a new notification service with WebSocket hub and message bus
func NewNotificationService(
	hub *Hub,
	mb *messagebus.MessageBus,
	opts ...Option,
) *NotificationService {
	s := &NotificationService{
		hub:  hub,
		mb:   mb,
		log:  slog.Default(),
		subs: make([]*nats.Subscription, 0),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithLogger sets the logger
func WithLogger(log *slog.Logger) Option {
	return func(s *NotificationService) { s.log = log }
}

// WithConfig sets the configuration
func WithConfig(cfg *config.Config) Option {
	return func(s *NotificationService) { s.cfg = cfg }
}

// Start initializes all NATS subscriptions for the notification service
func (s *NotificationService) Start(ctx context.Context) error {
	s.log.Info("Starting notification service subscriptions")

	if err := s.setupJobUpdateSubscription(); err != nil {
		return err
	}

	if err := s.setupTaskStatusSubscription(); err != nil {
		return err
	}

	if err := s.setupSubTaskSubscription(); err != nil {
		return err
	}

	s.log.Info("All NATS subscriptions established", slog.Int("count", len(s.subs)))
	return nil
}

// Stop unsubscribes from all NATS subscriptions
func (s *NotificationService) Stop() {
	s.log.Info("Stopping notification service", slog.Int("subscriptions", len(s.subs)))

	for _, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.log.Error("Failed to unsubscribe", slog.Any("error", err))
		}
	}

	s.subs = s.subs[:0] // Clear slice
}

// GetWebSocketHandler returns the WebSocket handler for HTTP routing
func (s *NotificationService) GetWebSocketHandler() *Handler {
	return NewHandler(s.hub, s.log)
}

// setupJobUpdateSubscription subscribes to job update messages and broadcasts them
func (s *NotificationService) setupJobUpdateSubscription() error {
	sub, err := s.mb.SubscribeToJobUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.JobUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			s.log.Error("Failed to unmarshal job update", slog.Any("error", err))
			return
		}

		s.log.Info("Broadcasting job update", slog.String("jobId", m.JobID))
		s.hub.Broadcast(m)
	})

	if err != nil {
		s.log.Error("Failed to subscribe to job updates", slog.Any("error", err))
		return err
	}

	s.subs = append(s.subs, sub)
	return nil
}

// setupTaskStatusSubscription subscribes to task status messages and broadcasts to job groups
func (s *NotificationService) setupTaskStatusSubscription() error {
	sub, err := s.mb.SubscribeToTaskStatusUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.TaskStatusUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			s.log.Error("Failed to unmarshal task update", slog.Any("error", err))
			return
		}

		s.log.Info("Broadcasting task status update", slog.String("jobId", m.JobID))
		s.hub.BroadcastToGroup(m, m.JobID)
	})

	if err != nil {
		s.log.Error("Failed to subscribe to task status updates", slog.Any("error", err))
		return err
	}

	s.subs = append(s.subs, sub)
	return nil
}

// setupSubTaskSubscription subscribes to subtask update messages and broadcasts to job groups
func (s *NotificationService) setupSubTaskSubscription() error {
	sub, err := s.mb.SubscribeToSubTaskUpdate(func(ctx context.Context, msg *nats.Msg) {
		var m messagebus.SubTaskUpdateMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			s.log.Error("Failed to unmarshal subtask update", slog.Any("error", err))
			return
		}

		s.log.Info("Broadcasting subtask update",
			slog.String("jobId", m.JobID),
			slog.String("key", m.Key),
			slog.String("status", string(m.SubTask.Status)),
			slog.String("url", m.SubTask.URL),
			slog.String("description", m.SubTask.Description))

		s.hub.BroadcastToGroup(m, m.JobID)
	})

	if err != nil {
		s.log.Error("Failed to subscribe to subtask updates", slog.Any("error", err))
		return err
	}

	s.subs = append(s.subs, sub)
	return nil
}
