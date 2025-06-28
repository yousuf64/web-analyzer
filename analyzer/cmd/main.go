package main

import (
	"analyzer/internal/analyzer"
	"analyzer/internal/config"
	"context"
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
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	cfg := config.Load()
	log := log.SetupFromEnv(cfg.Service.Name)

	log.Info("Starting analyzer service", slog.String("version", cfg.Service.Version))

	ctx := context.Background()
	shutdown, err := tracing.SetupOTelSDK(ctx, cfg.Tracing)
	if err != nil {
		log.Error("Failed to setup tracing", slog.Any("error", err))
		os.Exit(1)
	}
	defer shutdown(ctx)

	jobRepo, taskRepo, publisher, client, metrics, cleanup, err := initializeDependencies(cfg)
	if err != nil {
		log.Error("Failed to initialize dependencies", slog.Any("error", err))
		os.Exit(1)
	}
	defer cleanup()

	anlyzr := analyzer.NewAnalyzer(
		jobRepo,
		taskRepo,
		publisher,
		analyzer.WithHTTPClient(client),
		analyzer.WithMetrics(metrics),
		analyzer.WithLogger(log),
		analyzer.WithConfig(cfg),
	)

	sub, err := publisher.SubscribeToAnalyzeMessage(anlyzr.ProcessAnalyzeMessage)
	if err != nil {
		log.Error("Failed to subscribe to analyze message", slog.Any("error", err))
		os.Exit(1)
	}
	defer sub.Unsubscribe()

	log.Info("Analyzer service is running")

	waitForShutdown(log)
}

// initializeDependencies initializes individual dependencies
func initializeDependencies(cfg *config.Config) (
	*repository.JobRepository,
	*repository.TaskRepository,
	*messagebus.MessageBus,
	*http.Client,
	metrics.AnalyzerMetricsInterface,
	func(),
	error,
) {
	// Initialize metrics
	m := metrics.NewAnalyzerMetrics()
	m.MustRegisterAnalyzer()
	m.SetServiceInfo(cfg.Service.Version, runtime.Version())

	// Start metrics server
	srv := m.StartMetricsServer(cfg.Metrics.Port)

	// Initialize database
	ddc, err := repository.NewDynamoDBClient(cfg.DynamoDB)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	repository.SeedTables(ddc, cfg.DynamoDB, m)

	jobs, err := repository.NewJobRepository(cfg.DynamoDB, repository.WithJobMetrics(m))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	tasks, err := repository.NewTaskRepository(cfg.DynamoDB, repository.WithTaskMetrics(m))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	// Initialize HTTP client with tracing
	tr := http.DefaultTransport
	tr = tracing.HTTPClientMiddleware()(tr)

	client := &http.Client{
		Timeout:   cfg.HTTP.Timeout,
		Transport: tr,
	}

	// Initialize NATS connection
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	bus := messagebus.New(nc, m)

	cleanup := func() {
		nc.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if srv != nil {
			srv.Shutdown(ctx)
		}
	}

	return jobs, tasks, bus, client, m, cleanup, nil
}

// waitForShutdown waits for a shutdown signal
func waitForShutdown(log *slog.Logger) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	sig := <-ch

	log.Info("Shutting down analyzer service", slog.String("signal", sig.String()))
}
