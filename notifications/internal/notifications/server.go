package notifications

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"shared/config"
	"shared/middleware"
	"shared/tracing"
	"time"

	"github.com/yousuf64/shift"
)

// Server handles the HTTP server and notification service
type Server struct {
	srv             *http.Server
	notificationSvc *NotificationService
	log             *slog.Logger
	cfg             *config.HTTPServerConfig
}

// ServerOption configures the Server
type ServerOption func(*Server)

// NewServer creates a new server with notification service
func NewServer(
	notificationSvc *NotificationService,
	opts ...ServerOption,
) *Server {
	s := &Server{
		notificationSvc: notificationSvc,
		log:             slog.Default(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithServerConfig sets the server configuration
func WithServerConfig(cfg *config.HTTPServerConfig) ServerOption {
	return func(s *Server) { s.cfg = cfg }
}

// WithServerLogger sets the logger for the server
func WithServerLogger(log *slog.Logger) ServerOption {
	return func(s *Server) { s.log = log }
}

// Start starts the server and notification service
func (s *Server) Start(ctx context.Context) error {
	// Start notification service
	if err := s.notificationSvc.Start(ctx); err != nil {
		return err
	}

	// Setup router with middleware
	router := shift.New()
	router.Use(tracing.OtelMiddleware)
	router.Use(middleware.CORSMiddleware)
	router.Use(middleware.ErrorMiddleware(s.log))

	// Register routes
	router.OPTIONS("/*wildcard", middleware.OptionsHandler)
	router.GET("/ws", s.handleWebSocket)

	// Configure server
	addr := ":8081"
	if s.cfg != nil && s.cfg.Addr != "" {
		addr = s.cfg.Addr
	}

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      router.Serve(),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.log.Info("HTTP server starting", slog.String("addr", addr))
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down HTTP server")

	// Stop notification service
	s.notificationSvc.Stop()

	// Shutdown HTTP server
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}

	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request, route shift.Route) error {
	wsHandler := s.notificationSvc.GetWebSocketHandler()
	wsHandler.HandleWebSocket(w, r)
	return nil
}
