package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"shared/log"
	"shared/messagebus"
	"shared/metrics"
	"shared/repository"
	"shared/types"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/oklog/ulid/v2"
	"github.com/yousuf64/shift"
)

var (
	jobRepo    *repository.JobRepository
	taskRepo   *repository.TaskRepository
	logger     *slog.Logger
	mc         *metrics.APIMetrics
)

func main() {
	logger = log.SetupFromEnv("api")
	logger.Info("Starting API service")

	// Initialize metrics
	mc = metrics.NewAPIMetrics()
	mc.MustRegisterAPI()
	mc.SetServiceInfo("1.0.0", runtime.Version())

	// Start metrics server
	metricsServer := mc.StartMetricsServer("9090")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		metricsServer.Shutdown(ctx)
	}()

	dynamodb, err := repository.NewDynamoDBClient()
	if err != nil {
		logger.Error("Failed to create DynamoDB client", slog.Any("error", err))
		os.Exit(1)
	}

	if err := repository.SeedTables(dynamodb, mc); err != nil {
		logger.Error("Failed to seed tables", slog.Any("error", err))
		os.Exit(1)
	}

	jobRepo, err = repository.NewJobRepository(mc)
	if err != nil {
		logger.Error("Failed to create job repository", slog.Any("error", err))
		os.Exit(1)
	}

	taskRepo, err = repository.NewTaskRepository(mc)
	if err != nil {
		logger.Error("Failed to create task repository", slog.Any("error", err))
		os.Exit(1)
	}

	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", slog.Any("error", err))
		os.Exit(1)
	}
	defer nc.Close()

	mb := messagebus.New(nc, mc)

	router := shift.New()
	router.Use(corsMiddleware)
	router.Use(mc.HTTPMiddleware)
	router.Use(errorMiddleware)

	// Register OPTIONS handler for all routes, so that CORS is handled by the middleware
	router.OPTIONS("/*wildcard", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	router.POST("/analyze", func(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
		jobCreationStart := time.Now()
		defer func() {
			mc.RecordJobCreation(err == nil, time.Since(jobCreationStart))
		}()

		var req types.AnalyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return errors.Join(err, errors.New("failed to decode request"))
		}

		jobId := generateId()
		logger.Info("Creating new analysis job",
			slog.String("jobId", jobId),
			slog.String("url", req.Url))

		job := &types.Job{
			ID:        jobId,
			URL:       req.Url,
			Status:    types.JobStatusPending,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := jobRepo.CreateJob(job); err != nil {
			return errors.Join(err, errors.New("failed to create job"))
		}

		defaultTasks := getDefaultTasks(jobId)
		if err := taskRepo.CreateTasks(defaultTasks...); err != nil {
			return errors.Join(err, errors.New("failed to create tasks"))
		}

		if err := mb.PublishAnalyzeMessage(messagebus.AnalyzeMessage{
			Type:  messagebus.AnalyzeMessageType,
			JobId: jobId,
		}); err != nil {
			return errors.Join(err, errors.New("failed to publish analyze message"))
		}

		logger.Info("Analysis request published",
			slog.String("jobId", jobId),
			slog.String("url", req.Url))

		w.WriteHeader(http.StatusAccepted)
		return json.NewEncoder(w).Encode(types.AnalyzeResponse{
			Job: *job,
		})
	})

	router.GET("/jobs", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobs, err := jobRepo.GetAllJobs()
		if err != nil {
			return errors.Join(err, errors.New("failed to get jobs"))
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(jobs)
	})

	router.GET("/jobs/:job_id/tasks", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		jobId := route.Params.Get("job_id")
		if jobId == "" {
			return errors.New("job_id is required")
		}

		tasks, err := taskRepo.GetTasksByJobId(jobId)
		if err != nil {
			return errors.Join(err, errors.New("failed to get tasks"))
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(tasks)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router.Serve(),
	}

	go func() {
		logger.Info("API server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("Shutting down API service", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("Server gracefully stopped")
}

// entropyPool provides a pool of monotonic entropy sources for ULID generation
// This allows for better performance in concurrent scenarios by avoiding lock contention
var entropyPool = sync.Pool{
	New: func() any {
		return ulid.Monotonic(rand.Reader, 0)
	},
}

func generateId() string {
	e := entropyPool.Get().(*ulid.MonotonicEntropy)

	ts := ulid.Timestamp(time.Now())
	id := ulid.MustNew(ts, e)

	entropyPool.Put(e)
	return id.String()
}

func getDefaultTasks(jobId string) []*types.Task {
	return []*types.Task{
		{
			JobID:  jobId,
			Type:   types.TaskTypeExtracting,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeIdentifyingVersion,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeAnalyzing,
			Status: types.TaskStatusPending,
		},
		{
			JobID:  jobId,
			Type:   types.TaskTypeVerifyingLinks,
			Status: types.TaskStatusPending,
		},
	}
}
