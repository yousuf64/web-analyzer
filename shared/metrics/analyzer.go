package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	analyzerServiceName = "analyzer"
)

// AnalyzerMetricsInterface is an interface for analyzer metrics
type AnalyzerMetricsInterface interface {
	MustRegisterAnalyzer()
	RecordAnalysisJob(success bool, duration float64)
	RecordAnalysisTask(taskType string, success bool, duration float64)
	RecordLinkVerification(success bool, duration float64)
	RecordHTTPClientRequest(statusCode int, duration float64, method, requestType string)
	SetConcurrentLinkVerifications(count int)
}

// NoOpAnalyzerMetrics is a no-op implementation of AnalyzerMetricsInterface
type NoOpAnalyzerMetrics struct{}

// NewNoOpAnalyzerMetrics creates a new no-op analyzer metrics
func NewNoOpAnalyzerMetrics() AnalyzerMetricsInterface {
	return &NoOpAnalyzerMetrics{}
}

func (n *NoOpAnalyzerMetrics) MustRegisterAnalyzer()                       {}
func (n *NoOpAnalyzerMetrics) SetServiceInfo(version, goVersion string)    {}
func (n *NoOpAnalyzerMetrics) StartMetricsServer(port string) *http.Server { return nil }
func (n *NoOpAnalyzerMetrics) RecordAnalysisJob(success bool, duration float64) {
}
func (n *NoOpAnalyzerMetrics) RecordAnalysisTask(taskType string, success bool, duration float64) {}
func (n *NoOpAnalyzerMetrics) RecordLinkVerification(success bool, duration float64) {
}
func (n *NoOpAnalyzerMetrics) RecordHTTPClientRequest(statusCode int, duration float64, method, requestType string) {
}
func (n *NoOpAnalyzerMetrics) SetConcurrentLinkVerifications(count int) {}

type AnalyzerMetrics struct {
	*ServiceMetrics

	AnalysisJobsProcessedTotal  *prometheus.CounterVec
	AnalysisDuration            *prometheus.HistogramVec
	AnalysisTasksCompletedTotal *prometheus.CounterVec
	AnalysisTaskDuration        *prometheus.HistogramVec

	LinksVerifiedTotal          *prometheus.CounterVec
	LinkVerificationDuration    *prometheus.HistogramVec
	ConcurrentLinkVerifications prometheus.Gauge

	HTTPClientRequestsTotal   *prometheus.CounterVec
	HTTPClientRequestDuration *prometheus.HistogramVec
}

// NewAnalyzerMetrics creates a new analyzer metrics
func NewAnalyzerMetrics() *AnalyzerMetrics {
	baseMetrics := NewServiceMetrics(analyzerServiceName)

	analyzerMetrics := &AnalyzerMetrics{
		ServiceMetrics: baseMetrics,

		AnalysisJobsProcessedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "analysis_jobs_processed_total",
				Help:        "Total number of analysis jobs processed",
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{LabelStatus},
		),

		AnalysisDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "analysis_duration_seconds",
				Help:        "Total analysis time per job in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{},
		),

		AnalysisTasksCompletedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "analysis_tasks_completed_total",
				Help:        "Total number of analysis tasks completed",
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{LabelTaskType, LabelStatus},
		),

		AnalysisTaskDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "analysis_task_duration_seconds",
				Help:        "Analysis task processing time in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{LabelTaskType},
		),

		LinksVerifiedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "links_verified_total",
				Help:        "Total number of links verified",
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{"link_status"},
		),

		LinkVerificationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "link_verification_duration_seconds",
				Help:        "Link verification time in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{"outcome"},
		),

		ConcurrentLinkVerifications: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "concurrent_link_verifications",
				Help:        "Current number of concurrent link verifications",
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
		),

		HTTPClientRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "http_client_requests_total",
				Help:        "Total number of outbound HTTP requests",
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{LabelStatus, LabelMethod, LabelRequestType},
		),

		HTTPClientRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "http_client_request_duration_seconds",
				Help:        "HTTP client request duration in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: analyzerServiceName},
			},
			[]string{LabelMethod, LabelRequestType},
		),
	}

	return analyzerMetrics
}

// MustRegisterAnalyzer registers the analyzer metrics and base service metrics
func (m *AnalyzerMetrics) MustRegisterAnalyzer() {
	m.ServiceMetrics.MustRegister()

	prometheus.MustRegister(
		m.AnalysisJobsProcessedTotal,
		m.AnalysisDuration,
		m.AnalysisTasksCompletedTotal,
		m.AnalysisTaskDuration,
		m.LinksVerifiedTotal,
		m.LinkVerificationDuration,
		m.ConcurrentLinkVerifications,
		m.HTTPClientRequestsTotal,
		m.HTTPClientRequestDuration,
	)
}

// RecordAnalysisJob records the analysis job metrics
func (m *AnalyzerMetrics) RecordAnalysisJob(success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}

	m.AnalysisJobsProcessedTotal.WithLabelValues(status).Inc()
	m.AnalysisDuration.WithLabelValues().Observe(duration)
}

// RecordAnalysisTask records the analysis task metrics
func (m *AnalyzerMetrics) RecordAnalysisTask(taskType string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	m.AnalysisTasksCompletedTotal.WithLabelValues(taskType, status).Inc()
	m.AnalysisTaskDuration.WithLabelValues(taskType).Observe(duration)
}

// RecordLinkVerification records the link verification metrics
func (m *AnalyzerMetrics) RecordLinkVerification(success bool, duration float64) {
	outcome := "success"
	if !success {
		outcome = "failed"
	}

	m.LinksVerifiedTotal.WithLabelValues(outcome).Inc()
	m.LinkVerificationDuration.WithLabelValues(outcome).Observe(duration)
}

// RecordHTTPClientRequest records the HTTP client request metrics
func (m *AnalyzerMetrics) RecordHTTPClientRequest(status int, duration float64, method, requestType string) {
	m.HTTPClientRequestsTotal.WithLabelValues(strconv.Itoa(status), method, requestType).Inc()
	m.HTTPClientRequestDuration.WithLabelValues(method, requestType).Observe(duration)
}

// SetConcurrentLinkVerifications sets the concurrent link verifications metrics
func (m *AnalyzerMetrics) SetConcurrentLinkVerifications(count int) {
	m.ConcurrentLinkVerifications.Set(float64(count))
}
