package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"shared/messagebus"
	"shared/repository"
	"shared/types"
	"syscall"

	"github.com/nats-io/nats.go"
)

var jobRepo *repository.JobRepository
var taskRepo *repository.TaskRepository
var nc *nats.Conn
var mb *messagebus.MessageBus

func updateJobStatus(jobId string, status types.JobStatus) {
	err := jobRepo.UpdateJobStatus(jobId, status)
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
	dynamodb, err := repository.NewDynamoDBClient()
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client %v", err)
	}
	repository.SeedTables(dynamodb)

	jobRepo, err = repository.NewJobRepository()
	if err != nil {
		log.Fatalf("Failed to create jobRepository %v", err)
	}

	taskRepo, err = repository.NewTaskRepository()
	if err != nil {
		log.Fatalf("Failed to create taskRepository %v", err)
	}

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	mb = messagebus.New(nc)

	sub, err := nc.Subscribe("url.analyze", func(msg *nats.Msg) {
		var am types.AnalyzeMessage
		if err := json.Unmarshal(msg.Data, &am); err != nil {
			log.Printf("Failed to unmarshal: %v", err)
			return
		}

		processMessage(am)
	})
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

func processMessage(am types.AnalyzeMessage) {
	job, err := jobRepo.GetJob(am.JobId)
	if err != nil {
		log.Fatalf("Failed to get job %v", err)
	}

	updateJobStatus(am.JobId, types.JobStatusRunning)

	r, err := http.NewRequest(http.MethodGet, job.URL, nil)
	if err != nil {
		log.Fatalf("Failed to create request %v", err)
		updateJobStatus(am.JobId, types.JobStatusFailed)
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Fatalf("HTTP request failed %v", err)
		updateJobStatus(am.JobId, types.JobStatusFailed)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read the response body %v", err)
		updateJobStatus(am.JobId, types.JobStatusFailed)
	}

	a := NewAnalyzer(func(taskType types.TaskType, status types.TaskStatus) {
		err := taskRepo.UpdateTaskStatus(am.JobId, taskType, status)
		if err != nil {
			log.Printf("Update task status failed: %v", err)
			return
		}

		mb.PublishTaskStatusUpdate(messagebus.TaskStatusUpdateMessage{
			Type:     messagebus.TaskStatusUpdateMessageType,
			JobID:    am.JobId,
			TaskType: string(taskType),
			Status:   string(status),
		})
	}, func(taskType types.TaskType, key string, status types.TaskStatus) {
		err := taskRepo.UpdateSubTaskStatusByKey(am.JobId, taskType, key, status)
		if err != nil {
			log.Printf("Update subtask status failed: %v", err)
			return
		}

		mb.PublishSubTaskStatusUpdate(messagebus.SubTaskStatusUpdateMessage{
			Type:     messagebus.SubTaskStatusUpdateMessageType,
			JobID:    am.JobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(status),
		})
	}, func(taskType types.TaskType, key, url string) {
		log.Printf("Adding subtask for URL %v with key %v", url, key)
		err := taskRepo.AddSubTaskByKey(am.JobId, taskType, key, types.SubTask{
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
			JobID:    am.JobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(types.TaskStatusPending),
			URL:      url,
		})
	})
	res, err := a.AnalyzeHTML(string(b))
	if err != nil {
		log.Fatalf("Failed to analyze HTML %v", err)
		updateJobStatus(am.JobId, types.JobStatusFailed)
	}

	completedStatus := types.JobStatusCompleted
	err = jobRepo.UpdateJob(job.ID, &completedStatus, &res)
	if err != nil {
		log.Fatalf("Failed updating job %v", err)
		updateJobStatus(am.JobId, types.JobStatusFailed)
	}

	updateJobStatus(am.JobId, types.JobStatusCompleted)
}
