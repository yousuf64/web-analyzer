package api

import (
	"log/slog"
	"net/http"

	"github.com/yousuf64/shift"
)

// corsMiddleware handles CORS requests
func (a *API) corsMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		return next(w, r, route)
	}
}

// errorMiddleware handles errors
func (a *API) errorMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		err := next(w, r, route)
		if err != nil {
			a.log.Error("Request error",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Any("error", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}
}
