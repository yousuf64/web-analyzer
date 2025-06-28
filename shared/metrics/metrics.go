package metrics

import (
	"net/http"
	"strconv"
	"time"

	"shared/middleware"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yousuf64/shift"
)

const (
	LabelService     = "service"
	LabelMethod      = "method"
	LabelEndpoint    = "endpoint"
	LabelStatus      = "status"
	LabelOperation   = "operation"
	LabelTable       = "table"
	LabelJobStatus   = "job_status"
	LabelTaskType    = "task_type"
	LabelMessageType = "message_type"
	LabelRequestType = "request_type"
)

type ServiceMetrics struct {
	// HTTP
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight *prometheus.GaugeVec

	// System
	ServiceUptime prometheus.Gauge
	ServiceInfo   *prometheus.GaugeVec

	// Message bus
	NATSMessagesPublished *prometheus.CounterVec
	NATSMessagesReceived  *prometheus.CounterVec
	NATSMessageDuration   *prometheus.HistogramVec

	// Database
	DatabaseOperationsTotal   *prometheus.CounterVec
	DatabaseOperationDuration *prometheus.HistogramVec

	uptimeTicker *time.Ticker
}

func NewServiceMetrics(serviceName string) *ServiceMetrics {
	metrics := &ServiceMetrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "http_requests_total",
				Help:        "Total number of HTTP requests",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMethod, LabelEndpoint, LabelStatus},
		),

		HTTPRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "http_request_duration_seconds",
				Help:        "HTTP request duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMethod, LabelEndpoint},
		),

		HTTPRequestsInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:        "http_requests_in_flight",
				Help:        "Current number of HTTP requests being served",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMethod, LabelEndpoint},
		),

		ServiceUptime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "service_uptime_seconds",
				Help:        "Service uptime in seconds",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
		),

		ServiceInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name:        "service_info",
				Help:        "Service information",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{"version", "go_version"},
		),

		NATSMessagesPublished: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "nats_messages_published_total",
				Help:        "Total number of NATS messages published",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMessageType, LabelStatus},
		),

		NATSMessagesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "nats_messages_received_total",
				Help:        "Total number of NATS messages received",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMessageType, LabelStatus},
		),

		NATSMessageDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "nats_message_processing_duration_seconds",
				Help:        "NATS message processing duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelMessageType},
		),

		DatabaseOperationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "database_operations_total",
				Help:        "Total number of database operations",
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelOperation, LabelTable, LabelStatus},
		),

		DatabaseOperationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "database_operation_duration_seconds",
				Help:        "Database operation duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: serviceName},
			},
			[]string{LabelOperation, LabelTable},
		),
	}

	return metrics
}

func (m *ServiceMetrics) MustRegister() {
	prometheus.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPRequestsInFlight,
		m.ServiceUptime,
		m.ServiceInfo,
		m.NATSMessagesPublished,
		m.NATSMessagesReceived,
		m.NATSMessageDuration,
		m.DatabaseOperationsTotal,
		m.DatabaseOperationDuration,
	)
}

func (m *ServiceMetrics) HTTPMiddleware(next shift.HandlerFunc) shift.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		start := time.Now()

		m.HTTPRequestsInFlight.WithLabelValues(r.Method, r.URL.Path).Inc()
		defer m.HTTPRequestsInFlight.WithLabelValues(r.Method, r.URL.Path).Dec()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		err := next(wrapped, r, route)

		status := strconv.Itoa(wrapped.statusCode)

		m.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(time.Since(start).Seconds())

		return err
	}
}

func (m *ServiceMetrics) RecordNATSPublish(messageType string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	m.NATSMessagesPublished.WithLabelValues(messageType, status).Inc()
}

func (m *ServiceMetrics) RecordNATSReceive(messageType string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	m.NATSMessagesReceived.WithLabelValues(messageType, status).Inc()
	m.NATSMessageDuration.WithLabelValues(messageType).Observe(duration.Seconds())
}

func (m *ServiceMetrics) RecordDatabaseOperation(operation, table string, start time.Time, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.DatabaseOperationsTotal.WithLabelValues(operation, table, status).Inc()
	m.DatabaseOperationDuration.WithLabelValues(operation, table).Observe(time.Since(start).Seconds())
}

func (m *ServiceMetrics) SetServiceInfo(version, goVersion string) {
	m.ServiceInfo.WithLabelValues(version, goVersion).Set(1)
}

func (m *ServiceMetrics) startUptimeTracking() {
	startTime := time.Now()

	m.ServiceUptime.Set(0)
	m.uptimeTicker = time.NewTicker(30 * time.Second)

	go func() {
		for range m.uptimeTicker.C {
			m.ServiceUptime.Set(time.Since(startTime).Seconds())
		}
	}()
}

func (m *ServiceMetrics) stopUptimeTracking() {
	if m.uptimeTicker != nil {
		m.uptimeTicker.Stop()
		m.uptimeTicker = nil
	}
}

func (m *ServiceMetrics) StartMetricsServer(port string) *http.Server {
	router := shift.New()
	router.Use(middleware.CORSMiddleware)

	router.GET("/metrics", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		promhttp.Handler().ServeHTTP(w, r)
		return nil
	})

	router.GET("/health", func(w http.ResponseWriter, r *http.Request, route shift.Route) error {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return nil
	})

	// Handle OPTIONS for CORS preflight
	router.OPTIONS("/*wildcard", middleware.OptionsHandler)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router.Serve(),
	}

	server.RegisterOnShutdown(func() {
		m.stopUptimeTracking()
	})

	go func() {
		m.startUptimeTracking()

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic("Failed to start metrics server: " + err.Error())
		}
	}()

	return server
}

// responseWriter wraps [http.ResponseWriter] to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
