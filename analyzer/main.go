package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"shared/log"
	"shared/messagebus"
	"shared/metrics"
	"shared/repository"
	"shared/tracing"
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
	mc     *metrics.AnalyzerMetrics
)

func updateJobStatus(ctx context.Context, jobId string, status types.JobStatus) error {
	err := jb.UpdateJobStatus(ctx, jobId, status)
	if err != nil {
		return err
	}

	if err := mb.PublishJobUpdate(ctx, messagebus.JobUpdateMessage{
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

	ctx := context.Background()
	shutdown, err := tracing.SetupOTelSDK(ctx, "analyzer")
	if err != nil {
		logger.Error("Failed to setup tracing", slog.Any("error", err))
		os.Exit(1)
	}
	defer shutdown(ctx)

	// Initialize metrics
	mc = metrics.NewAnalyzerMetrics()
	mc.MustRegisterAnalyzer()
	mc.SetServiceInfo("1.0.0", runtime.Version())

	// Start metrics server
	metricsServer := mc.StartMetricsServer("9091")
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		metricsServer.Shutdown(ctx)
	}()

	// Initialize dependencies
	ddc, err := repository.NewDynamoDBClient()
	if err != nil {
		logger.Error("Failed to create DynamoDB client", slog.Any("error", err))
		os.Exit(1)
	}
	repository.SeedTables(ddc, mc)

	jb, err = repository.NewJobRepository(mc)
	if err != nil {
		logger.Error("Failed to create job repository", slog.Any("error", err))
		os.Exit(1)
	}

	tsk, err = repository.NewTaskRepository(mc)
	if err != nil {
		logger.Error("Failed to create task repository", slog.Any("error", err))
		os.Exit(1)
	}

	tr := http.DefaultTransport
	tr = tracing.HTTPClientMiddleware()(tr)

	h = &http.Client{
		Timeout:   httpClientTimeout,
		Transport: tr,
	}

	nc, err = nats.Connect(nats.DefaultURL)
	if err != nil {
		logger.Error("Failed to connect to NATS", slog.Any("error", err))
		os.Exit(1)
	}
	defer nc.Close()

	mb = messagebus.New(nc, mc)

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

func messageHandler(ctx context.Context, msg *nats.Msg) {
	var am messagebus.AnalyzeMessage
	if err := json.Unmarshal(msg.Data, &am); err != nil {
		logger.Error("Failed to unmarshal analyze message",
			slog.Any("error", err),
			slog.String("data", string(msg.Data)))
		return
	}

	logger.Info("Processing analyze request", slog.String("jobId", am.JobId))

	analysisStart := time.Now()
	err := analyzeUrl(ctx, am)
	if err != nil {
		logger.Error("Failed to process analyze request",
			slog.String("jobId", am.JobId),
			slog.Any("error", err))
		mc.RecordAnalysisJob(false, time.Since(analysisStart).Seconds())
		return
	}

	duration := time.Since(analysisStart)
	logger.Info("Completed analyze request",
		slog.String("jobId", am.JobId),
		slog.Duration("processingTime", duration))

	mc.RecordAnalysisJob(true, duration.Seconds())
}

func analyzeUrl(ctx context.Context, am messagebus.AnalyzeMessage) (err error) {
	defer func() {
		if err != nil {
			logger.Error("Analysis failed",
				slog.String("jobId", am.JobId),
				slog.Any("error", err))

			if err := updateJobStatus(ctx, am.JobId, types.JobStatusFailed); err != nil {
				logger.Error("Failed to update job status",
					slog.String("jobId", am.JobId),
					slog.Any("error", err))
			}

			tsk.UpdateTaskStatus(ctx, am.JobId, types.TaskTypeExtracting, types.TaskStatusFailed)
			tsk.UpdateTaskStatus(ctx, am.JobId, types.TaskTypeIdentifyingVersion, types.TaskStatusFailed)
			tsk.UpdateTaskStatus(ctx, am.JobId, types.TaskTypeAnalyzing, types.TaskStatusFailed)
			tsk.UpdateTaskStatus(ctx, am.JobId, types.TaskTypeVerifyingLinks, types.TaskStatusFailed)
		}
	}()

	job, err := jb.GetJob(ctx, am.JobId)
	if err != nil {
		return errors.Join(err, errors.New("job not found"))
	}

	logger.Info("Starting analysis",
		slog.String("jobId", am.JobId),
		slog.String("url", job.URL))

	if err := updateJobStatus(ctx, am.JobId, types.JobStatusRunning); err != nil {
		return errors.Join(err, errors.New("failed to update job status"))
	}

	c, err := fetchContent(ctx, job.URL)
	if err != nil {
		return errors.Join(err, errors.New("failed to fetch content"))
	}

	an := NewAnalyzer(h, mc)
	an.SetBaseUrl(job.URL)
	an.TaskStatusUpdateCallback = updateTaskStatus(ctx, am.JobId)
	an.AddSubTaskCallback = addSubTask(ctx, am.JobId)
	an.SubTaskUpdateCallback = updateSubTaskStatus(ctx, am.JobId)

	res, err := an.AnalyzeHTML(ctx, c)
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
	err = jb.UpdateJob(ctx, job.ID, &completedStatus, &res)
	if err != nil {
		return errors.Join(err, errors.New("failed to update job"))
	}

	err = mb.PublishJobUpdate(ctx, messagebus.JobUpdateMessage{
		Type:   messagebus.JobUpdateMessageType,
		JobID:  am.JobId,
		Status: string(types.JobStatusCompleted),
		Result: &res,
	})
	return err
}

func fetchContent(ctx context.Context, url string) (string, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to create request"))
	}

	start := time.Now()
	resp, err := h.Do(r)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to execute request"))
	}
	defer resp.Body.Close()

	mc.RecordHTTPClientRequest(resp.StatusCode, time.Since(start).Seconds(), r.Method, "content_fetch")

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Join(err, errors.New("failed to read response body"))
	}

	return string(b), nil
}

func updateTaskStatus(ctx context.Context, jobId string) TaskStatusUpdateCallback {
	return func(taskType types.TaskType, status types.TaskStatus) {
		err := tsk.UpdateTaskStatus(ctx, jobId, taskType, status)
		if err != nil {
			logger.Error("Failed to update task status",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("status", string(status)),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishTaskStatusUpdate(ctx, messagebus.TaskStatusUpdateMessage{
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

func addSubTask(ctx context.Context, jobId string) AddSubTaskCallback {
	return func(taskType types.TaskType, key, url string) {
		subtask := types.SubTask{
			Type:        types.SubTaskTypeValidatingLink,
			Status:      types.TaskStatusPending,
			URL:         url,
			Description: "",
		}

		err := tsk.AddSubTaskByKey(ctx, jobId, taskType, key, subtask)
		if err != nil {
			logger.Error("Failed to add subtask",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("url", url),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishSubTaskUpdate(ctx, messagebus.SubTaskUpdateMessage{
			Type:     messagebus.SubTaskUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			SubTask:  subtask,
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

func updateSubTaskStatus(ctx context.Context, jobId string) SubTaskUpdateCallback {
	return func(taskType types.TaskType, key string, subtask types.SubTask) {
		err := tsk.UpdateSubTaskByKey(ctx, jobId, taskType, key, subtask)
		if err != nil {
			logger.Error("Failed to update subtask",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("url", subtask.URL),
				slog.String("description", subtask.Description),
				slog.String("status", string(subtask.Status)),
				slog.Any("error", err))
			return
		}

		if err := mb.PublishSubTaskUpdate(ctx, messagebus.SubTaskUpdateMessage{
			Type:     messagebus.SubTaskUpdateMessageType,
			JobID:    jobId,
			TaskType: string(taskType),
			Key:      key,
			SubTask:  subtask,
		}); err != nil {
			logger.Error("Failed to publish subtask status update",
				slog.String("jobId", jobId),
				slog.String("taskType", string(taskType)),
				slog.String("key", key),
				slog.String("url", subtask.URL),
				slog.String("description", subtask.Description),
				slog.String("status", string(subtask.Status)),
				slog.Any("error", err))
		}
	}
}
