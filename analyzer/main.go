package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"shared/messagebus"
	"shared/repository"
	"shared/types"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

const httpClientTimeout = 20 * time.Second

var (
	jb  *repository.JobRepository
	tsk *repository.TaskRepository
	nc  *nats.Conn
	mb  *messagebus.MessageBus
	h   *http.Client
)

func updateJobStatus(jobId string, status types.JobStatus) {
	err := jb.UpdateJobStatus(jobId, status)
	if err != nil {
		log.Printf("Failed to update job status: %v", err)
		return
	}

	mb.PublishJobUpdate(messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  jobId,
		Status: string(status),
		Result: nil,
	})
}

func main() {
	ddc, err := repository.NewDynamoDBClient()
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client %v", err)
	}
	repository.SeedTables(ddc)

	jb, err = repository.NewJobRepository()
	if err != nil {
		log.Fatalf("Failed to create jobRepository %v", err)
	}

	tsk, err = repository.NewTaskRepository()
	if err != nil {
		log.Fatalf("Failed to create taskRepository %v", err)
	}

	h = &http.Client{
		Timeout: httpClientTimeout,
	}

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	mb = messagebus.New(nc)
	sub, err := mb.SubscribeToAnalyzeMessage(messageHandler)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	log.Println("Analyzer service is running...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down analyzer...")
}

func messageHandler(msg *nats.Msg) {
	var am types.AnalyzeMessage
	if err := json.Unmarshal(msg.Data, &am); err != nil {
		log.Printf("Failed to unmarshal: %v", err)
		return
	}

	err := analyze(am)
	if err != nil {
		log.Printf("Failed to process message: %v", err)
		return
	}
}

func analyze(am types.AnalyzeMessage) (err error) {
	defer func() {
		if err != nil {
			updateJobStatus(am.JobId, types.JobStatusFailed)
		}
	}()

	job, err := jb.GetJob(am.JobId)
	if err != nil {
		return errors.Join(err, errors.New("job not found"))
	}

	updateJobStatus(am.JobId, types.JobStatusRunning)

	c, err := fetchContent(job.URL)
	if err != nil {
		return errors.Join(err, errors.New("failed to fetch content"))
	}

	an := NewAnalyzer()
	an.TaskStatusUpdateCallback = updateTaskStatus(am.JobId)
	an.AddSubTaskCallback = addSubTask(am.JobId)
	an.SubTaskStatusUpdateCallback = updateSubTaskStatus(am.JobId)

	res, err := an.AnalyzeHTML(c)
	if err != nil {
		return errors.Join(err, errors.New("failed to analyze HTML"))
	}

	completedStatus := types.JobStatusCompleted
	err = jb.UpdateJob(job.ID, &completedStatus, &res)
	if err != nil {
		return errors.Join(err, errors.New("failed to update job"))
	}
	return nil
}

func fetchContent(url string) (string, error) {
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to create request"))
	}

	resp, err := h.Do(r)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to execute request"))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to read response body"))
	}

	return string(b), nil
}

func updateTaskStatus(jobId string) TaskStatusUpdateCallback {
	return func(taskType types.TaskType, status types.TaskStatus) {
		err := tsk.UpdateTaskStatus(jobId, taskType, status)
		if err != nil {
			return
		}

		mb.PublishTaskStatusUpdate(messagebus.TaskStatusUpdateMessage{
			Type:     messagebus.TaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Status:   string(status),
		})
	}
}

func addSubTask(jobId string) AddSubTaskCallback {
	return func(taskType types.TaskType, key, url string) {
		log.Printf("Adding subtask for URL %v with key %v", url, key)
		err := tsk.AddSubTaskByKey(jobId, taskType, key, types.SubTask{
			Type:   types.SubTaskTypeValidatingLink,
			Status: types.TaskStatusPending,
			URL:    url,
		})
		if err != nil {
			log.Printf("Add subtask failed: %v", err)
			return
		}

		mb.PublishSubTaskStatusUpdate(messagebus.SubTaskStatusUpdateMessage{
			Type:     messagebus.SubTaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(types.TaskStatusPending),
			URL:      url,
		})
	}

}
func updateSubTaskStatus(jobId string) SubTaskStatusUpdateCallback {
	return func(taskType types.TaskType, key string, status types.TaskStatus) {
		err := tsk.UpdateSubTaskStatusByKey(jobId, taskType, key, status)
		if err != nil {
			log.Printf("Update subtask status failed: %v", err)
			return
		}

		mb.PublishSubTaskStatusUpdate(messagebus.SubTaskStatusUpdateMessage{
			Type:     messagebus.SubTaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(status),
		})
	}
}
