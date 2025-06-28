package notifications

import (
	"context"
	"log/slog"
	"net/http"
	"shared/config"
	"time"
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
func (s *Server) Start() error {
	// Start notification service
	ctx := context.Background()
	if err := s.notificationSvc.Start(ctx); err != nil {
		return err
	}

	// Setup routes
	mux := http.NewServeMux()
	wsHandler := s.notificationSvc.GetWebSocketHandler()
	mux.HandleFunc("/ws", s.corsMiddleware(wsHandler.HandleWebSocket))

	// Configure server
	addr := ":8081"
	if s.cfg != nil && s.cfg.Addr != "" {
		addr = s.cfg.Addr
	}

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
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

// corsMiddleware adds CORS headers to responses
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}
