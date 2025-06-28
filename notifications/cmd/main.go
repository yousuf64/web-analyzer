package main

import (
	"context"
	"log/slog"
	"notifications/internal/config"
	"notifications/internal/notifications"
	"os"
	"os/signal"
	"runtime"
	"shared/log"
	"shared/messagebus"
	"shared/metrics"
	"shared/tracing"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg := config.Load()

	// Setup logging
	logger := log.SetupFromEnv(cfg.Service.Name)
	logger.Info("Starting notifications service")

	// Setup tracing
	otelShutdown, err := tracing.SetupOTelSDK(ctx, cfg.Service.Name)
	if err != nil {
		logger.Error("Failed to setup OTel SDK", slog.Any("error", err))
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

	// Create notification service
	notificationService := notifications.NewNotificationService(
		deps.Hub,
		deps.MessageBus,
		notifications.WithLogger(logger),
		notifications.WithConfig(cfg),
	)

	// Create and start server
	srv := notifications.NewServer(
		notificationService,
		notifications.WithServerConfig(&cfg.HTTP),
		notifications.WithServerLogger(logger),
	)

	// Start server in goroutine
	go func() {
		logger.Info("Starting notification server", slog.String("addr", cfg.HTTP.Addr))
		if err := srv.Start(); err != nil {
			logger.Error("Failed to start server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down notification service...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown server gracefully", slog.Any("error", err))
	}

	logger.Info("Notification service stopped")
}

type dependencies struct {
	Hub        *notifications.Hub
	MessageBus *messagebus.MessageBus
	Metrics    *metrics.NotificationsMetrics
	NC         *nats.Conn
}

func initializeDependencies(cfg *config.Config, logger *slog.Logger) (*dependencies, func(), error) {
	// Initialize metrics
	m := metrics.NewNotificationsMetrics()
	m.MustRegisterNotifications()
	m.SetServiceInfo(cfg.Service.Version, runtime.Version())

	// Start metrics server
	metricsServer := m.StartMetricsServer(cfg.Metrics.Port)

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, nil, err
	}

	// Create message bus
	mb := messagebus.New(nc, m)

	// Create WebSocket hub
	hub := notifications.NewHub(
		notifications.WithHubMetrics(m),
		notifications.WithHubLogger(logger),
	)

	deps := &dependencies{
		Hub:        hub,
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

		// Close WebSocket hub
		hub.Close()
	}

	return deps, cleanup, nil
}
