package messagebus

import (
	"context"
	"encoding/json"
	"log"
	"shared/models"
	"shared/tracing"
	"time"

	"github.com/nats-io/nats.go"
)

//go:generate mockgen -destination=../mocks/mock_messagebus.go -package=mocks . MessageBusInterface

type MessageBusInterface interface {
	PublishAnalyzeMessage(ctx context.Context, m AnalyzeMessage) error
	PublishJobUpdate(ctx context.Context, m JobUpdateMessage) error
	PublishTaskStatusUpdate(ctx context.Context, m TaskStatusUpdateMessage) error
	PublishSubTaskUpdate(ctx context.Context, m SubTaskUpdateMessage) error
	SubscribeToAnalyzeMessage(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error)
	SubscribeToJobUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error)
	SubscribeToTaskStatusUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error)
	SubscribeToSubTaskUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error)
}

type MessageType string

const (
	AnalyzeMessageType          MessageType = "url.analyze"
	JobUpdateMessageType        MessageType = "job.update"
	TaskStatusUpdateMessageType MessageType = "task.status_update"
	SubTaskUpdateMessageType    MessageType = "task.subtask_update"
)

type AnalyzeMessage struct {
	Type  MessageType `json:"type"`
	JobId string      `json:"job_id"`
}

type JobUpdateMessage struct {
	Type   MessageType           `json:"type"`
	JobID  string                `json:"job_id"`
	Status string                `json:"status"`
	Result *models.AnalyzeResult `json:"result,omitempty"`
}

type TaskStatusUpdateMessage struct {
	Type     MessageType `json:"type"`
	JobID    string      `json:"job_id"`
	TaskType string      `json:"task_type"`
	Status   string      `json:"status"`
}

type SubTaskUpdateMessage struct {
	Type     MessageType    `json:"type"`
	JobID    string         `json:"job_id"`
	TaskType string         `json:"task_type"`
	Key      string         `json:"key"`
	SubTask  models.SubTask `json:"subtask"`
}

// MessageBus provides a NATS message bus for publishing and subscribing to messages
type MessageBus struct {
	nc      *nats.Conn
	metrics MetricsCollector
}

// New creates a new message bus
func New(nc *nats.Conn, metrics MetricsCollector) *MessageBus {
	if metrics == nil {
		metrics = NoOpMetricsCollector{}
	}
	return &MessageBus{
		nc:      nc,
		metrics: metrics,
	}
}

func (b *MessageBus) PublishAnalyzeMessage(ctx context.Context, m AnalyzeMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(AnalyzeMessageType), err == nil)
	}()

	m.Type = AnalyzeMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal analyze message: %v", err)
		return err
	}

	err = b.publishMsg(ctx, data, AnalyzeMessageType)
	if err != nil {
		log.Printf("Failed to publish analyze message: %v", err)
	}
	return err
}

// PublishJobUpdate publishes a job update message to NATS
func (b *MessageBus) PublishJobUpdate(ctx context.Context, m JobUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(JobUpdateMessageType), err == nil)
	}()

	m.Type = JobUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal job update: %v", err)
		return err
	}

	err = b.publishMsg(ctx, data, JobUpdateMessageType)
	if err != nil {
		log.Printf("Failed to publish job update: %v", err)
	}
	return err
}

// PublishTaskStatusUpdate publishes a task status update message to NATS
func (b *MessageBus) PublishTaskStatusUpdate(ctx context.Context, m TaskStatusUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(TaskStatusUpdateMessageType), err == nil)
	}()

	m.Type = TaskStatusUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal task status update: %v", err)
		return err
	}

	err = b.publishMsg(ctx, data, TaskStatusUpdateMessageType)
	if err != nil {
		log.Printf("Failed to publish task status update: %v", err)
	}
	return err
}

// PublishSubTaskUpdate publishes a subtask update message to NATS
func (b *MessageBus) PublishSubTaskUpdate(ctx context.Context, m SubTaskUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(SubTaskUpdateMessageType), err == nil)
	}()

	m.Type = SubTaskUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal subtask update: %v", err)
		return err
	}

	err = b.publishMsg(ctx, data, SubTaskUpdateMessageType)
	if err != nil {
		log.Printf("Failed to publish subtask update: %v", err)
	}
	return err
}

// publishMsg publishes a message to NATS with trace context in headers
func (b *MessageBus) publishMsg(ctx context.Context, data []byte, messageType MessageType) (err error) {
	ctx, span := tracing.CreateNATSPublishSpan(ctx, string(messageType))
	defer span.End()

	msg := &nats.Msg{
		Subject: string(messageType),
		Data:    data,
		Header:  make(nats.Header),
	}

	tracing.InjectNATSHeaders(ctx, msg)

	err = b.nc.PublishMsg(msg)
	if err != nil {
		tracing.SetError(ctx, err)
	}
	return err
}

// SubscribeToAnalyzeMessage subscribes to the analyze message
func (b *MessageBus) SubscribeToAnalyzeMessage(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(AnalyzeMessageType, handler)
	return b.nc.Subscribe(string(AnalyzeMessageType), h)
}

// SubscribeToJobUpdate subscribes to the job update message
func (b *MessageBus) SubscribeToJobUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(JobUpdateMessageType, handler)
	return b.nc.Subscribe(string(JobUpdateMessageType), h)
}

// SubscribeToTaskStatusUpdate subscribes to the task status update message
func (b *MessageBus) SubscribeToTaskStatusUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(TaskStatusUpdateMessageType, handler)
	return b.nc.Subscribe(string(TaskStatusUpdateMessageType), h)
}

// SubscribeToSubTaskUpdate subscribes to the subtask update message
func (b *MessageBus) SubscribeToSubTaskUpdate(handler func(ctx context.Context, m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(SubTaskUpdateMessageType, handler)
	return b.nc.Subscribe(string(SubTaskUpdateMessageType), h)
}

// wrapHandler wraps the original handler to automatically inject trace context and record receive metrics
func (b *MessageBus) wrapHandler(messageType MessageType, handler func(ctx context.Context, m *nats.Msg)) nats.MsgHandler {
	return func(m *nats.Msg) {
		ctx := tracing.ExtractNATSHeaders(context.Background(), m)
		ctx, span := tracing.CreateNATSConsumeSpan(ctx, m.Subject)
		defer span.End()

		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				// If handler panics, record as error
				b.metrics.RecordNATSReceive(string(messageType), time.Since(start), false)
				panic(r)
			} else {
				// Record successful processing
				b.metrics.RecordNATSReceive(string(messageType), time.Since(start), true)
			}
		}()

		handler(ctx, m)
	}
}
