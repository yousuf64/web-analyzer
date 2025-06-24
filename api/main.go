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

func corsMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		return next(w, r, route)
	}
}

func errorMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		err := next(w, r, route)
		if err != nil {
			log.Printf("Error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}
}

func main() {
	dynamodb, err := repository.NewDynamoDBClient()
	if err != nil {
		log.Fatalf("Failed to create DynamoDB client %v", err)
	}
	repository.SeedTables(dynamodb)

	jobRepo, err = repository.NewJobRepository()
	if err != nil {
		log.Fatalf("Failed to create job repo %v", err)
	}

	taskRepo, err = repository.NewTaskRepository()
	if err != nil {
		log.Fatalf("Failed to create task repo %v", err)
	}

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	router := shift.New()
	router.Use(corsMiddleware)
	router.Use(errorMiddleware)

	// Register OPTIONS handler for all routes, so that CORS is handled by the middleware
	router.OPTIONS("/*wildcard", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	router.POST("/analyze", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		var req types.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return err
		}

		jobId := strconv.Itoa(int(time.Now().UnixNano()))
		job := &types.Job{
			ID:        jobId,
			URL:       req.Url,
			Status:    types.JobStatusPending,
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		}
		err := jobRepo.CreateJob(job)

		if err != nil {
			return err
		}

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
		return json.NewEncoder(w).Encode(types.AnalyzeResponse{
			Job: *job,
		})
	})

	router.GET("/jobs", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobs, err := jobRepo.GetAllJobs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(jobs)
	})

	router.GET("/jobs/:job_id/tasks", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobId := route.Params.Get("job_id")
		if jobId == "" {
			http.Error(w, "Job ID is required", http.StatusBadRequest)
			return nil
		}

		tasks, err := taskRepo.GetTasksByJobId(jobId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return err
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(tasks)
	})

	log.Printf("API server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", router.Serve()))
}
