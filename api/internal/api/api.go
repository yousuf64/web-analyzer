package api

import (
	"api/internal/config"
	"context"
	"log/slog"
	"net"
	"net/http"
	"shared/messagebus"
	"shared/metrics"
	"shared/repository"
	"shared/tracing"
	"shared/types"
	"time"

	"github.com/yousuf64/shift"
)

// API handles the HTTP server and routes
type API struct {
	jobRepo  *repository.JobRepository
	taskRepo *repository.TaskRepository
	mb       *messagebus.MessageBus
	metrics  *metrics.APIMetrics
	log      *slog.Logger
	srv      *http.Server
}

// AnalyzeRequest is the request body for the analyze endpoint
type AnalyzeRequest struct {
	URL string `json:"url"`
}

// AnalyzeResponse is the response body for the analyze endpoint
type AnalyzeResponse struct {
	Job types.Job `json:"job"`
}

// NewAPI creates a new API with all dependencies
func NewAPI(
	jobRepo *repository.JobRepository,
	taskRepo *repository.TaskRepository,
	mb *messagebus.MessageBus,
	metrics *metrics.APIMetrics,
	log *slog.Logger,
) *API {
	return &API{
		jobRepo:  jobRepo,
		taskRepo: taskRepo,
		mb:       mb,
		metrics:  metrics,
		log:      log,
	}
}

// Start starts the HTTP server
func (a *API) Start(ctx context.Context, cfg *config.Config) error {
	router := shift.New()
	router.Use(tracing.OtelMiddleware)
	router.Use(a.corsMiddleware)
	if a.metrics != nil {
		router.Use(a.metrics.HTTPMiddleware)
	}
	router.Use(a.errorMiddleware)

	// Register routes
	router.OPTIONS("/*wildcard", a.handleOptions)
	router.POST("/analyze", a.handleAnalyze)
	router.GET("/jobs", a.handleGetJobs)
	router.GET("/jobs/:job_id/tasks", a.handleGetTasksByJobID)

	addr := ":8080"
	if cfg != nil && cfg.HTTP.Addr != "" {
		addr = cfg.HTTP.Addr
	}

	a.srv = &http.Server{
		Addr:         addr,
		Handler:      router.Serve(),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	a.log.Info("API server starting", slog.String("addr", addr))
	return a.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (a *API) Shutdown(ctx context.Context) error {
	a.log.Info("Shutting down API server")
	if a.srv != nil {
		return a.srv.Shutdown(ctx)
	}
	return nil
}
