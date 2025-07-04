package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"shared/messagebus"
	"shared/models"
	"strings"
	"time"

	"github.com/yousuf64/shift"
)

// handleAnalyze handles the analyze endpoint
func (a *API) handleAnalyze(w http.ResponseWriter, r *http.Request, _ shift.Route) error {
	ctx := r.Context()
	start := time.Now()

	var success bool
	defer func() {
		if a.metrics != nil {
			a.metrics.RecordJobCreation(success, time.Since(start))
		}
	}()

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.Join(err, errors.New("failed to decode request"))
	}

	// Validate and normalize the URL
	validatedURL, err := validateURL(req.URL)
	if err != nil {
		http.Error(w, "Invalid URL, please check the URL and try again.", http.StatusBadRequest)
		return nil
	}

	jobID := generateID()
	a.log.Info("Creating new analysis job",
		slog.String("jobId", jobID),
		slog.String("url", validatedURL))

	job := &models.Job{
		ID:        jobID,
		URL:       validatedURL,
		Status:    models.JobStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := a.jobRepo.CreateJob(ctx, job); err != nil {
		return errors.Join(err, errors.New("failed to create job"))
	}

	defaultTasks := getDefaultTasks(jobID)
	if err := a.taskRepo.CreateTasks(ctx, defaultTasks...); err != nil {
		return errors.Join(err, errors.New("failed to create tasks"))
	}

	if err := a.mb.PublishAnalyzeMessage(ctx, messagebus.AnalyzeMessage{
		Type:  messagebus.AnalyzeMessageType,
		JobId: jobID,
	}); err != nil {
		return errors.Join(err, errors.New("failed to publish analyze message"))
	}

	a.log.Info("Analysis request published",
		slog.String("jobId", jobID),
		slog.String("url", validatedURL),
		slog.Duration("duration", time.Since(start)))

	success = true
	w.WriteHeader(http.StatusAccepted)
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(AnalyzeResponse{Job: *job})
}

// handleGetJobs handles the get jobs endpoint
func (a *API) handleGetJobs(w http.ResponseWriter, r *http.Request, route shift.Route) error {
	ctx := r.Context()

	jobs, err := a.jobRepo.GetAllJobs(ctx)
	if err != nil {
		return errors.Join(err, errors.New("failed to get jobs"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(jobs)
}

// handleGetTasksByJobID handles the get tasks by job ID endpoint
func (a *API) handleGetTasksByJobID(w http.ResponseWriter, r *http.Request, route shift.Route) error {
	ctx := r.Context()
	jobID := route.Params.Get("job_id")

	if strings.TrimSpace(jobID) == "" {
		return errors.New("job_id is required")
	}

	tasks, err := a.taskRepo.GetTasksByJobId(ctx, jobID)
	if err != nil {
		return errors.Join(err, errors.New("failed to get tasks"))
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(tasks)
}
