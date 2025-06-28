package main

import (
	"api/internal/api"
	"api/internal/config"
	"context"
	"log/slog"
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
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	// Setup logging
	logger := log.SetupFromEnv(cfg.Service.Name)
	logger.Info("Starting API service")

	// Setup tracing
	otelShutdown, err := tracing.SetupOTelSDK(ctx, cfg.Tracing)
	if err != nil {
		logger.Error("Failed to setup tracing", slog.Any("error", err))
		os.Exit(1)
	}
	defer otelShutdown(ctx)

	// Initialize dependencies
	deps, cleanup, err := initializeDependencies(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize dependencies", slog.Any("error", err))
		os.Exit(1)
	}
	defer cleanup()

	// Create API service
	apiService := api.NewAPI(
		deps.JobRepo,
		deps.TaskRepo,
		deps.MessageBus,
		deps.Metrics,
		logger,
	)

	// Start server in goroutine
	go func() {
		logger.Info("Starting API server", slog.String("addr", cfg.HTTP.Addr))
		if err := apiService.Start(ctx, cfg); err != nil {
			logger.Error("Failed to start server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("Shutting down API service", slog.String("signal", sig.String()))

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := apiService.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown server gracefully", slog.Any("error", err))
	}

	logger.Info("API service stopped")
}

type dependencies struct {
	JobRepo    *repository.JobRepository
	TaskRepo   *repository.TaskRepository
	MessageBus *messagebus.MessageBus
	Metrics    *metrics.APIMetrics
	NC         *nats.Conn
}

func initializeDependencies(cfg *config.Config, logger *slog.Logger) (*dependencies, func(), error) {
	// Initialize metrics
	m := metrics.NewAPIMetrics()
	m.MustRegisterAPI()

	// Get service info from environment
	m.SetServiceInfo(cfg.Service.Version, runtime.Version())

	// Start metrics server
	metricsServer := m.StartMetricsServer(cfg.Metrics.Port)

	// Initialize DynamoDB client
	dynamodb, err := repository.NewDynamoDBClient(cfg.DynamoDB)
	if err != nil {
		return nil, nil, err
	}

	// Seed tables
	if err := repository.SeedTables(dynamodb, cfg.DynamoDB, m); err != nil {
		return nil, nil, err
	}

	// Create repositories
	jobRepo, err := repository.NewJobRepository(cfg.DynamoDB, m)
	if err != nil {
		return nil, nil, err
	}

	taskRepo, err := repository.NewTaskRepository(cfg.DynamoDB, m)
	if err != nil {
		return nil, nil, err
	}

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, nil, err
	}

	// Create message bus
	mb := messagebus.New(nc, m)

	deps := &dependencies{
		JobRepo:    jobRepo,
		TaskRepo:   taskRepo,
		MessageBus: mb,
		Metrics:    m,
		NC:         nc,
	}

	cleanup := func() {
		logger.Info("Cleaning up dependencies")

		// Shutdown metrics server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown metrics server", slog.Any("error", err))
		}

		// Close NATS connection
		nc.Close()
	}

	return deps, cleanup, nil
}
