package main

import (
	"encoding/json"
	"log"
	"net/http"
	"shared/repository"
	"shared/types"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/yousuf64/shift"
)

var jobRepo *repository.JobRepository
var taskRepo *repository.TaskRepository

func main() {
	dynamodb, err := repository.NewDynamoDBClient()
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client %v", err)
	}
	repository.SeedTables(dynamodb)

	jobRepo, err := repository.NewJobRepository()
	if err != nil {
		log.Fatalf("Failed to create job repo %v", err)
	}

	taskRepo, err := repository.NewTaskRepository()
	if err != nil {
		log.Fatalf("Failed to create task repo %v", err)
	}

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	router := shift.New()
	router.POST("/analyze", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		var req types.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		jobId := strconv.Itoa(int(time.Now().UnixNano()))
		jobRepo.CreateJob(&types.Job{
			ID:          jobId,
			URL:         req.Url,
			Status:      types.JobStatusPending,
			CreatedAt:   time.Time{},
			UpdatedAt:   time.Time{},
			StartedAt:   nil,
			CompletedAt: nil,
			Result:      nil,
		})

		err = taskRepo.CreateTasks(&types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeExtracting,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeIdentifyingVersion,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeAnalyzing,
			Status: types.TaskStatusPending,
		}, &types.Task{
			JobID:  jobId,
			Type:   types.TaskTypeVerifyingLinks,
			Status: types.TaskStatusPending,
		})
		if err != nil {
			return err
		}

		msg, err := json.Marshal(types.AnalyzeMessage{
			JobId: jobId,
		})
		if err != nil {
			return err
		}

		err = nc.Publish("url.analyze", msg)
		if err != nil {
			return err
		}
		log.Printf("Message published: %v", req)

		w.WriteHeader(http.StatusAccepted)
		return nil
	})

	log.Printf("API server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router.Serve()))
}
