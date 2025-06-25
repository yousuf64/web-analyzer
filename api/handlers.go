package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"shared/messagebus"
	"shared/types"

	"github.com/oklog/ulid/v2"
	"github.com/yousuf64/shift"
)

func handleAnalyze(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	ctx := r.Context()

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

	if err := jobRepo.CreateJob(ctx, job); err != nil {
		return errors.Join(err, errors.New("failed to create job"))
	}

	defaultTasks := getDefaultTasks(jobId)
	if err := taskRepo.CreateTasks(ctx, defaultTasks...); err != nil {
		return errors.Join(err, errors.New("failed to create tasks"))
	}

	if err := mb.PublishAnalyzeMessage(ctx, messagebus.AnalyzeMessage{
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

}

func handleGetJobs(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	jobs, err := jobRepo.GetAllJobs(r.Context())
	if err != nil {
		return errors.Join(err, errors.New("failed to get jobs"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(jobs)
}

func handleGetTasksByJobId(w http.ResponseWriter, r *http.Request, route shift.Route) (err error) {
	jobId := route.Params.Get("job_id")
	if jobId == "" {
		return errors.New("job_id is required")
	}

	tasks, err := taskRepo.GetTasksByJobId(r.Context(), jobId)
	if err != nil {
		return errors.Join(err, errors.New("failed to get tasks"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(tasks)
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
