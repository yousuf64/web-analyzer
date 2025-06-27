package tracing

import (
	"net/http"
	"strconv"

	"github.com/yousuf64/shift"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// OtelMiddleware creates a standard HTTP tracing middleware
func OtelMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		ctx := GetPropagator().Extract(r.Context(), &httpHeaderCarrier{r.Header})

		spanName := r.Method + " " + route.Path
		ctx, span := StartSpan(ctx, spanName)
		defer span.End()

		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.URLPath(r.URL.Path),
			semconv.URLFull(r.URL.String()),
			semconv.URLScheme(r.URL.Scheme),
			semconv.HTTPRoute(route.Path),
			attribute.String("http.user_agent", r.UserAgent()),
		)

		if r.ContentLength > 0 {
			span.SetAttributes(semconv.HTTPRequestBodySize(int(r.ContentLength)))
		}

		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		r = r.WithContext(ctx)
		err := next(w, r, route)

		span.SetAttributes(semconv.HTTPResponseStatusCode(wrapped.statusCode))

		if wrapped.statusCode >= 400 || err != nil {
			span.SetStatus(codes.Error, "HTTP "+strconv.Itoa(wrapped.statusCode))
		}

		return err
	}
}

// HTTPClientMiddleware creates tracing for outbound HTTP requests
func HTTPClientMiddleware() func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return &tracingRoundTripper{next: next}
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// tracingRoundTripper implements http.RoundTripper with tracing
type tracingRoundTripper struct {
	next http.RoundTripper
}

func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Create span for outbound request
	ctx, span := StartSpan(ctx, "http.client.request")
	defer span.End()

	// Set HTTP client attributes
	span.SetAttributes(
		semconv.HTTPRequestMethodKey.String(req.Method),
		semconv.URLFull(req.URL.String()),
		semconv.URLScheme(req.URL.Scheme),
		attribute.String("network.peer.name", req.URL.Hostname()),
	)

	if req.URL.Port() != "" {
		span.SetAttributes(attribute.String("network.peer.port", req.URL.Port()))
	}

	// Inject trace context into outbound request
	GetPropagator().Inject(ctx, &httpHeaderCarrier{req.Header})

	// Update request context
	req = req.WithContext(ctx)

	// Execute request
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		SetError(ctx, err)
		return resp, err
	}

	// Set response attributes
	span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))

	// Mark as error if HTTP error status
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, "HTTP "+strconv.Itoa(resp.StatusCode))
	}

	return resp, nil
}

// httpHeaderCarrier implements TextMapCarrier for HTTP headers
type httpHeaderCarrier struct {
	header http.Header
}

func (h *httpHeaderCarrier) Get(key string) string {
	return h.header.Get(key)
}

func (h *httpHeaderCarrier) Set(key, value string) {
	h.header.Set(key, value)
}

func (h *httpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(h.header))
	for k := range h.header {
		keys = append(keys, k)
	}
	return keys
}
