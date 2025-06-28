package middleware

import (
	"log/slog"
	"net/http"

	"github.com/yousuf64/shift"
)

// CORSMiddleware handles CORS requests with default settings
func CORSMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		return next(w, r, route)
	}
}

// ErrorMiddleware handles errors with structured logging
func ErrorMiddleware(logger *slog.Logger) func(shift.HandlerFunc) shift.HandlerFunc {
	return func(next shift.HandlerFunc) shift.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
			err := next(w, r, route)
			if err != nil {
				logger.Error("Request error",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("error", err))
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return err
		}
	}
}

// OptionsHandler handles OPTIONS requests for CORS preflight
// This can be used as a route handler for "/*wildcard" OPTIONS routes
func OptionsHandler(w http.ResponseWriter, r *http.Request, route shift.Route) error {
	w.WriteHeader(http.StatusOK)
	return nil
}
