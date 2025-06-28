package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"shared/messagebus"
	"shared/models"
	"time"

	"github.com/nats-io/nats.go"
)

// ProcessAnalyzeMessage handles incoming analyze messages
func (s *Analyzer) ProcessAnalyzeMessage(ctx context.Context, msg *nats.Msg) {
	var am messagebus.AnalyzeMessage
	if err := json.Unmarshal(msg.Data, &am); err != nil {
		s.log.Error("Failed to unmarshal analyze message",
			slog.Any("error", err),
			slog.String("data", string(msg.Data)))
		return
	}

	s.log.Info("Processing analyze request", slog.String("jobId", am.JobId))

	start := time.Now()
	err := s.analyzeURL(ctx, am)
	if err != nil {
		s.log.Error("Failed to process analyze request",
			slog.String("jobId", am.JobId),
			slog.Any("error", err))
		s.metrics.RecordAnalysisJob(false, time.Since(start).Seconds())
		return
	}

	d := time.Since(start)
	s.log.Info("Completed analyze request",
		slog.String("jobId", am.JobId),
		slog.Duration("processingTime", d))

	s.metrics.RecordAnalysisJob(true, d.Seconds())
}

// analyzeURL performs the complete URL analysis workflow
func (s *Analyzer) analyzeURL(ctx context.Context, am messagebus.AnalyzeMessage) error {
	job, err := s.jobRepo.GetJob(ctx, am.JobId)
	if err != nil {
		s.failAllTasks(ctx, am.JobId)
		return fmt.Errorf("job not found: %w", err)
	}

	s.log.Info("Starting analysis",
		slog.String("jobId", am.JobId),
		slog.String("url", job.URL))

	if err := s.updateJobStatus(ctx, am.JobId, models.JobStatusRunning); err != nil {
		s.failAllTasks(ctx, am.JobId)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	content, err := s.fetchContent(ctx, job.URL)
	if err != nil {
		s.failAllTasks(ctx, am.JobId)
		return fmt.Errorf("failed to fetch content: %w", err)
	}

	result, err := s.performAnalysis(ctx, am.JobId, job.URL, content)
	if err != nil {
		s.failAllTasks(ctx, am.JobId)
		return fmt.Errorf("failed to analyze HTML: %w", err)
	}

	return s.completeJob(ctx, *job, result)
}

// performAnalysis creates and runs the HTML analyzer
func (s *Analyzer) performAnalysis(ctx context.Context, jobID, url, content string) (models.AnalyzeResult, error) {
	result := &AnalysisResult{
		headings: make(map[string]int),
		links:    []string{},
		baseURL:  url,
	}

	if err := s.analyzeHTML(ctx, jobID, content, result); err != nil {
		return models.AnalyzeResult{}, err
	}

	return s.buildResult(result), nil
}

// updateJobStatus updates job status and publishes update
func (s *Analyzer) updateJobStatus(ctx context.Context, jobID string, status models.JobStatus) error {
	if err := s.jobRepo.UpdateJobStatus(ctx, jobID, status); err != nil {
		return err
	}

	return s.publisher.PublishJobUpdate(ctx, messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  jobID,
		Status: string(status),
		Result: nil,
	})
}

// completeJob finalizes the job with results
func (s *Analyzer) completeJob(ctx context.Context, job models.Job, result models.AnalyzeResult) error {
	s.log.Info("HTML analysis completed",
		slog.String("jobId", job.ID),
		slog.String("htmlVersion", result.HtmlVersion),
		slog.Int("linkCount", len(result.Links)),
		slog.Int("internalLinks", result.InternalLinkCount),
		slog.Int("externalLinks", result.ExternalLinkCount),
		slog.Int("accessibleLinks", result.AccessibleLinks),
		slog.Int("inaccessibleLinks", result.InaccessibleLinks),
		slog.Bool("hasLoginForm", result.HasLoginForm))

	completedStatus := models.JobStatusCompleted
	if err := s.jobRepo.UpdateJob(ctx, job.ID, &completedStatus, &result); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return s.publisher.PublishJobUpdate(ctx, messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  job.ID,
		Status: string(models.JobStatusCompleted),
		Result: &result,
	})
}

// failAllTasks marks all tasks as failed
func (s *Analyzer) failAllTasks(ctx context.Context, jobID string) {
	s.updateJobStatus(ctx, jobID, models.JobStatusFailed)
	s.taskRepo.UpdateTaskStatus(ctx, jobID, models.TaskTypeExtracting, models.TaskStatusFailed)
	s.taskRepo.UpdateTaskStatus(ctx, jobID, models.TaskTypeIdentifyingVersion, models.TaskStatusFailed)
	s.taskRepo.UpdateTaskStatus(ctx, jobID, models.TaskTypeAnalyzing, models.TaskStatusFailed)
	s.taskRepo.UpdateTaskStatus(ctx, jobID, models.TaskTypeVerifyingLinks, models.TaskStatusFailed)
}

// updateTaskStatus updates task status and publishes update
func (s *Analyzer) updateTaskStatus(ctx context.Context, jobID string, taskType models.TaskType, status models.TaskStatus) {
	if err := s.taskRepo.UpdateTaskStatus(ctx, jobID, taskType, status); err != nil {
		s.log.Error("Failed to update task status",
			slog.String("jobId", jobID),
			slog.String("taskType", string(taskType)),
			slog.String("status", string(status)),
			slog.Any("error", err))
	}

	if err := s.publisher.PublishTaskStatusUpdate(ctx, messagebus.TaskStatusUpdateMessage{
		Type:     messagebus.TaskStatusUpdateMessageType,
		JobID:    jobID,
		TaskType: string(taskType),
		Status:   string(status),
	}); err != nil {
		s.log.Error("Failed to publish task status update",
			slog.String("jobId", jobID),
			slog.String("taskType", string(taskType)),
			slog.String("status", string(status)),
			slog.Any("error", err))
	}
}
