package tracing

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/yousuf64/shift"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func GetTracer() trace.Tracer {
	return tracer
}

// GetPropagator returns the configured text map propagator
func GetPropagator() propagation.TextMapPropagator {
	return otel.GetTextMapPropagator()
}

// StartSpan creates a new span with the given name
func StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return GetTracer().Start(ctx, spanName)
}

// SetError marks the current span as having an error
func SetError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span != nil && err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTracerProvider(serviceName)
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)
	tracer = tracerProvider.Tracer(serviceName)

	// Set up meter provider.
	// meterProvider, err := newMeterProvider()
	// if err != nil {
	// 	handleErr(err)
	// 	return
	// }
	// shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	// otel.SetMeterProvider(meterProvider)

	// // Set up logger provider.
	// loggerProvider, err := newLoggerProvider()
	// if err != nil {
	// 	handleErr(err)
	// 	return
	// }
	// shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	// global.SetLoggerProvider(loggerProvider)

	return
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTracerProvider(serviceName string) (*sdktrace.TracerProvider, error) {
	zipkinExporter, err := zipkin.New(
		"http://localhost:9411/api/v2/spans",
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironmentName("development"),
		),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(zipkinExporter,
			sdktrace.WithBatchTimeout(time.Second*5)),
		sdktrace.WithResource(res),
	)
	return tracerProvider, nil
}

func newMeterProvider() (*metric.MeterProvider, error) {
	metricExporter, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			// Default is 1m. Set to 3s for demonstrative purposes.
			metric.WithInterval(3*time.Second))),
	)
	return meterProvider, nil
}

func newLoggerProvider() (*log.LoggerProvider, error) {
	logExporter, err := stdoutlog.New()
	if err != nil {
		return nil, err
	}

	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter)),
	)
	return loggerProvider, nil
}

// =============================================================================
// HTTP Middleware Support
// =============================================================================

// HTTPMiddleware creates a standard HTTP tracing middleware

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

// =============================================================================
// NATS Message Support
// =============================================================================

// InjectNATSHeaders injects trace context into NATS message headers
func InjectNATSHeaders(ctx context.Context, msg *nats.Msg) {
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	GetPropagator().Inject(ctx, &natsHeaderCarrier{msg.Header})
}

// ExtractNATSHeaders extracts trace context from NATS message headers
func ExtractNATSHeaders(ctx context.Context, msg *nats.Msg) context.Context {
	if msg.Header == nil {
		return ctx
	}
	return GetPropagator().Extract(ctx, &natsHeaderCarrier{msg.Header})
}

// CreateNATSPublishSpan creates a span for NATS message publishing
func CreateNATSPublishSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "messagebus.publish "+subject)
	span.SetAttributes(
		attribute.String("messagebus.system", "nats"),
		attribute.String("messagebus.operation", "publish"),
		attribute.String("messagebus.destination", subject),
	)
	return ctx, span
}

// CreateNATSConsumeSpan creates a span for NATS message consumption
func CreateNATSConsumeSpan(ctx context.Context, subject string) (context.Context, trace.Span) {
	ctx, span := StartSpan(ctx, "messagebus.consume "+subject)
	span.SetAttributes(
		attribute.String("messagebus.system", "nats"),
		attribute.String("messagebus.operation", "consume"),
		attribute.String("messagebus.source", subject),
	)
	return ctx, span
}

// =============================================================================
// Database Support
// =============================================================================

type DatabaseSpan struct {
	trace.Span

	Close func(error)
}

// CreateDatabaseSpan creates a span for database operations and returns a function to close the span
func CreateDatabaseSpan(ctx context.Context, operation, table string) (context.Context, *DatabaseSpan) {
	ctx, span := StartSpan(ctx, "db."+operation)
	span.SetAttributes(
		attribute.String("db.system", "dynamodb"),
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
	)
	return ctx, &DatabaseSpan{
		Span: span,
		Close: func(err error) {
			if err != nil {
				SetError(ctx, err)
			}
			span.End()
		},
	}
}

// =============================================================================
// Carrier Implementations
// =============================================================================

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

// natsHeaderCarrier implements TextMapCarrier for NATS headers
type natsHeaderCarrier struct {
	header nats.Header
}

func (n *natsHeaderCarrier) Get(key string) string {
	return n.header.Get(key)
}

func (n *natsHeaderCarrier) Set(key, value string) {
	n.header.Set(key, value)
}

func (n *natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(n.header))
	for k := range n.header {
		keys = append(keys, k)
	}
	return keys
}

// =============================================================================
// HTTP Response Writer Wrapper
// =============================================================================

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

// =============================================================================
// HTTP Client Wrapper
// =============================================================================

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
