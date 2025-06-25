package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"shared/log"
	"shared/messagebus"
	"shared/metrics"
	"shared/repository"
	"shared/tracing"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/yousuf64/shift"
)

var (
	jobRepo  *repository.JobRepository
	taskRepo *repository.TaskRepository
	logger   *slog.Logger
	mc       *metrics.APIMetrics
	mb       *messagebus.MessageBus
)

func main() {
	logger = log.SetupFromEnv("api")
	logger.Info("Starting API service")

	ctx := context.Background()
	otelShutdown, err := tracing.SetupOTelSDK(ctx, "api")
	if err != nil {
		logger.Error("Failed to setup tracing", slog.Any("error", err))
		os.Exit(1)
	}
	defer otelShutdown(ctx)

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

	mb = messagebus.New(nc, mc)

	router := shift.New()
	router.Use(tracing.OtelMiddleware)
	router.Use(corsMiddleware)
	router.Use(mc.HTTPMiddleware)
	router.Use(errorMiddleware)

	// Register OPTIONS handler for all routes, so that CORS is handled by the middleware
	router.OPTIONS("/*wildcard", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	router.POST("/analyze", handleAnalyze)
	router.GET("/jobs", handleGetJobs)
	router.GET("/jobs/:job_id/tasks", handleGetTasksByJobId)

	srv := &http.Server{
		Addr:        ":8080",
		Handler:     router.Serve(),
		BaseContext: func(_ net.Listener) context.Context { return ctx },
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
