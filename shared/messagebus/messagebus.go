package messagebus

import (
	"encoding/json"
	"log"
	"shared/types"
	"time"

	"github.com/nats-io/nats.go"
)

type MetricsCollector interface {
	RecordNATSPublish(messageType string, success bool)
	RecordNATSReceive(messageType string, duration time.Duration, success bool)
}

type NoOpMetricsCollector struct{}

func (n NoOpMetricsCollector) RecordNATSPublish(messageType string, success bool) {}
func (n NoOpMetricsCollector) RecordNATSReceive(messageType string, duration time.Duration, success bool) {
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
	Type   MessageType          `json:"type"`
	JobID  string               `json:"job_id"`
	Status string               `json:"status"`
	Result *types.AnalyzeResult `json:"result,omitempty"`
}

type TaskStatusUpdateMessage struct {
	Type     MessageType `json:"type"`
	JobID    string      `json:"job_id"`
	TaskType string      `json:"task_type"`
	Status   string      `json:"status"`
}

type SubTaskUpdateMessage struct {
	Type     MessageType   `json:"type"`
	JobID    string        `json:"job_id"`
	TaskType string        `json:"task_type"`
	Key      string        `json:"key"`
	SubTask  types.SubTask `json:"subtask"`
}

type MessageBus struct {
	nc      *nats.Conn
	metrics MetricsCollector
}

func New(nc *nats.Conn, metrics MetricsCollector) *MessageBus {
	if metrics == nil {
		metrics = NoOpMetricsCollector{}
	}
	return &MessageBus{
		nc:      nc,
		metrics: metrics,
	}
}

func NewWithoutMetrics(nc *nats.Conn) *MessageBus {
	return New(nc, NoOpMetricsCollector{})
}

func (b *MessageBus) PublishAnalyzeMessage(m AnalyzeMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(AnalyzeMessageType), err == nil)
	}()

	m.Type = AnalyzeMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal analyze message: %v", err)
		return err
	}

	err = b.nc.Publish(string(AnalyzeMessageType), data)
	return err
}

func (b *MessageBus) PublishJobUpdate(m JobUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(JobUpdateMessageType), err == nil)
	}()

	m.Type = JobUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal job update: %v", err)
		return err
	}

	err = b.nc.Publish(string(JobUpdateMessageType), data)
	if err != nil {
		log.Printf("Failed to publish job update: %v", err)
	}
	return err
}

func (b *MessageBus) PublishTaskStatusUpdate(m TaskStatusUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(TaskStatusUpdateMessageType), err == nil)
	}()

	m.Type = TaskStatusUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal task status update: %v", err)
		return err
	}

	err = b.nc.Publish(string(TaskStatusUpdateMessageType), data)
	if err != nil {
		log.Printf("Failed to publish task status update: %v", err)
	}
	return err
}

func (b *MessageBus) PublishSubTaskUpdate(m SubTaskUpdateMessage) (err error) {
	defer func() {
		b.metrics.RecordNATSPublish(string(SubTaskUpdateMessageType), err == nil)
	}()

	m.Type = SubTaskUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal subtask update: %v", err)
		return err
	}

	err = b.nc.Publish(string(SubTaskUpdateMessageType), data)
	if err != nil {
		log.Printf("Failed to publish subtask update: %v", err)
	}
	return err
}

// wrapHandler wraps the original handler to automatically record receive metrics
func (b *MessageBus) wrapHandler(messageType MessageType, handler nats.MsgHandler) nats.MsgHandler {
	return func(m *nats.Msg) {
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

		handler(m)
	}
}

func (b *MessageBus) SubscribeToAnalyzeMessage(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(AnalyzeMessageType, handler)
	return b.nc.Subscribe(string(AnalyzeMessageType), h)
}

func (b *MessageBus) SubscribeToJobUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(JobUpdateMessageType, handler)
	return b.nc.Subscribe(string(JobUpdateMessageType), h)
}

func (b *MessageBus) SubscribeToTaskStatusUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(TaskStatusUpdateMessageType, handler)
	return b.nc.Subscribe(string(TaskStatusUpdateMessageType), h)
}

func (b *MessageBus) SubscribeToSubTaskUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	h := b.wrapHandler(SubTaskUpdateMessageType, handler)
	return b.nc.Subscribe(string(SubTaskUpdateMessageType), h)
}
