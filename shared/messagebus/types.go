package messagebus

import (
	"encoding/json"
	"log"
	"shared/types"

	"github.com/nats-io/nats.go"
)

type MessageType string

const (
	AnalyzeMessageType             MessageType = "url.analyze"
	JobUpdateMessageType           MessageType = "job.update"
	TaskStatusUpdateMessageType    MessageType = "task.status_update"
	SubTaskStatusUpdateMessageType MessageType = "task.subtask_status_update"
)

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

type SubTaskStatusUpdateMessage struct {
	Type     MessageType `json:"type"`
	JobID    string      `json:"job_id"`
	TaskType string      `json:"task_type"`
	Key      string      `json:"key"`
	Status   string      `json:"status"`
	URL      string      `json:"url,omitempty"`
}

type MessageBus struct {
	nc *nats.Conn
}

func New(nc *nats.Conn) *MessageBus {
	return &MessageBus{nc: nc}
}

func (b *MessageBus) PublishJobUpdate(m JobUpdateMessage) error {
	m.Type = JobUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal job update: %v", err)
		return err
	}

	if err := b.nc.Publish(string(JobUpdateMessageType), data); err != nil {
		log.Printf("Failed to publish job update: %v", err)
		return err
	}

	return nil
}

func (b *MessageBus) PublishTaskStatusUpdate(m TaskStatusUpdateMessage) error {
	m.Type = TaskStatusUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal task status update: %v", err)
		return err
	}

	if err := b.nc.Publish(string(TaskStatusUpdateMessageType), data); err != nil {
		log.Printf("Failed to publish task status update: %v", err)
		return err
	}

	return nil
}

func (b *MessageBus) PublishSubTaskStatusUpdate(m SubTaskStatusUpdateMessage) error {
	m.Type = SubTaskStatusUpdateMessageType
	data, err := json.Marshal(m)
	if err != nil {
		log.Printf("Failed to marshal subtask status update: %v", err)
		return err
	}

	if err := b.nc.Publish(string(SubTaskStatusUpdateMessageType), data); err != nil {
		log.Printf("Failed to publish subtask status update: %v", err)
		return err
	}

	return nil
}

func (b *MessageBus) SubscribeToAnalyzeMessage(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(string(AnalyzeMessageType), handler)
}

func (b *MessageBus) SubscribeToJobUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(string(JobUpdateMessageType), handler)
}

func (b *MessageBus) SubscribeToTaskStatusUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(string(TaskStatusUpdateMessageType), handler)
}

func (b *MessageBus) SubscribeToSubTaskStatusUpdate(handler func(m *nats.Msg)) (*nats.Subscription, error) {
	return b.nc.Subscribe(string(SubTaskStatusUpdateMessageType), handler)
}
