package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"shared/log"
	"shared/messagebus"
	"shared/repository"
	"shared/types"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	httpClientTimeout = 20 * time.Second
)

var (
	jb     *repository.JobRepository
	tsk    *repository.TaskRepository
	nc     *nats.Conn
	mb     *messagebus.MessageBus
	h      *http.Client
	logger *slog.Logger
)

func updateJobStatus(jobId string, status types.JobStatus) error {
	err := jb.UpdateJobStatus(jobId, status)
	if err != nil {
		return err
	}

	if err := mb.PublishJobUpdate(messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  jobId,
		Status: string(status),
		Result: nil,
	}); err != nil {
		return err
	}

	return nil
}

func main() {
	logger = log.SetupFromEnv("analyzer")
	logger.Info("Starting analyzer service")

	// Initialize dependencies
	ddc, err := repository.NewDynamoDBClient()
	if err != nil {
		logger.Error("Failed to create DynamoDB client", slog.Any("error", err))
		os.Exit(1)
	}
	repository.SeedTables(ddc)

	jb, err = repository.NewJobRepository()
	if err != nil {
		logger.Error("Failed to create job repository", slog.Any("error", err))
		os.Exit(1)
	}

	tsk, err = repository.NewTaskRepository()
	if err != nil {
		logger.Error("Failed to create task repository", slog.Any("error", err))
		os.Exit(1)
	}

	h = &http.Client{
		Timeout: httpClientTimeout,
	}

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", slog.Any("error", err))
		os.Exit(1)
	}
	defer nc.Close()

	mb = messagebus.New(nc)
	sub, err := mb.SubscribeToAnalyzeMessage(messageHandler)
	if err != nil {
		logger.Error("Failed to subscribe to analyze message", slog.Any("error", err))
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	logger.Info("Analyzer service is running")

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("Shutting down analyzer service", slog.String("signal", sig.String()))
}

func messageHandler(msg *nats.Msg) {
	var am messagebus.AnalyzeMessage
	if err := json.Unmarshal(msg.Data, &am); err != nil {
		logger.Error("Failed to unmarshal analyze message",
			slog.Any("error", err),
			slog.String("data", string(msg.Data)))
		return
	}

	logger.Info("Processing analyze request", slog.String("jobId", am.JobId))

	startTime := time.Now()
	if err := analyze(am); err != nil {
		logger.Error("Failed to process analyze request",
			slog.String("jobId", am.JobId),
			slog.Any("error", err))
		return
	}

	duration := time.Since(startTime)
	logger.Info("Completed analyze request",
		slog.String("jobId", am.JobId),
		slog.Duration("processingTime", duration))
}

func analyze(am messagebus.AnalyzeMessage) (err error) {
	defer func() {
		if err != nil {
			logger.Error("Analysis failed",
				slog.String("jobId", am.JobId),
				slog.Any("error", err))

			if err := updateJobStatus(am.JobId, types.JobStatusFailed); err != nil {
				logger.Error("Failed to update job status",
					slog.String("jobId", am.JobId),
					slog.Any("error", err))
			}
		}
	}()

	job, err := jb.GetJob(am.JobId)
	if err != nil {
		return errors.Join(err, errors.New("job not found"))
	}

	logger.Info("Starting analysis",
		slog.String("jobId", am.JobId),
		slog.String("url", job.URL))

	if err := updateJobStatus(am.JobId, types.JobStatusRunning); err != nil {
		return errors.Join(err, errors.New("failed to update job status"))
	}

	c, err := fetchContent(job.URL)
	if err != nil {
		return errors.Join(err, errors.New("failed to fetch content"))
	}

	an := NewAnalyzer()
	an.SetBaseUrl(job.URL)
	an.TaskStatusUpdateCallback = updateTaskStatus(am.JobId)
	an.AddSubTaskCallback = addSubTask(am.JobId)
	an.SubTaskStatusUpdateCallback = updateSubTaskStatus(am.JobId)

	res, err := an.AnalyzeHTML(c)
	if err != nil {
		return errors.Join(err, errors.New("failed to analyze HTML"))
	}

	logger.Info("HTML analysis completed",
		slog.String("jobId", am.JobId),
		slog.String("htmlVersion", res.HtmlVersion),
		slog.Int("linkCount", len(res.Links)),
		slog.Int("internalLinks", res.InternalLinkCount),
		slog.Int("externalLinks", res.ExternalLinkCount),
		slog.Int("accessibleLinks", res.AccessibleLinks),
		slog.Int("inaccessibleLinks", res.InaccessibleLinks),
		slog.Bool("hasLoginForm", res.HasLoginForm))

	completedStatus := types.JobStatusCompleted
	err = jb.UpdateJob(job.ID, &completedStatus, &res)
	if err != nil {
		return errors.Join(err, errors.New("failed to update job"))
	}

	return updateJobStatus(am.JobId, types.JobStatusCompleted)
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
	defer resp.Body.Close()

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
			logger.Error("Failed to update task status",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("status", string(status)),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishTaskStatusUpdate(messagebus.TaskStatusUpdateMessage{
			Type:     messagebus.TaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Status:   string(status),
		}); err != nil {
			logger.Error("Failed to publish task status update",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("status", string(status)),
				slog.Any("error", err))
		}
	}
}

func addSubTask(jobId string) AddSubTaskCallback {
	return func(taskType types.TaskType, key, url string) {
		err := tsk.AddSubTaskByKey(jobId, taskType, key, types.SubTask{
			Type:   types.SubTaskTypeValidatingLink,
			Status: types.TaskStatusPending,
			URL:    url,
		})
		if err != nil {
			logger.Error("Failed to add subtask",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("url", url),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishSubTaskStatusUpdate(messagebus.SubTaskStatusUpdateMessage{
			Type:     messagebus.SubTaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(types.TaskStatusPending),
			URL:      url,
		}); err != nil {
			logger.Error("Failed to publish subtask status update",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("status", string(types.TaskStatusPending)),
				slog.Any("error", err))
		}
	}
}

func updateSubTaskStatus(jobId string) SubTaskStatusUpdateCallback {
	return func(taskType types.TaskType, key string, status types.TaskStatus) {
		err := tsk.UpdateSubTaskStatusByKey(jobId, taskType, key, status)
		if err != nil {
			logger.Error("Failed to update subtask status",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("status", string(status)),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishSubTaskStatusUpdate(messagebus.SubTaskStatusUpdateMessage{
			Type:     messagebus.SubTaskStatusUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			Status:   string(status),
		}); err != nil {
			logger.Error("Failed to publish subtask status update",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("status", string(status)),
				slog.Any("error", err))
		}
	}
}
