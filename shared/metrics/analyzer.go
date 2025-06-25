package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	analyzerServiceName = "analyzer"
)

type AnalyzerMetricsInterface interface {
	MustRegisterAnalyzer()
	RecordAnalysisJob(success bool, duration float64)
	RecordAnalysisTask(taskType string, success bool, duration float64)
	RecordLinkVerification(success bool, duration float64)
	RecordHTTPClientRequest(statusCode int, duration float64, method, requestType string)
	SetConcurrentLinkVerifications(count int)
}

type NoopAnalyzerMetrics struct{}

func NewNoopAnalyzerMetrics() AnalyzerMetricsInterface {
	return &NoopAnalyzerMetrics{}
}

func (n *NoopAnalyzerMetrics) MustRegisterAnalyzer()                       {}
func (n *NoopAnalyzerMetrics) SetServiceInfo(version, goVersion string)    {}
func (n *NoopAnalyzerMetrics) StartMetricsServer(port string) *http.Server { return nil }
func (n *NoopAnalyzerMetrics) RecordAnalysisJob(success bool, duration float64) {
}
func (n *NoopAnalyzerMetrics) RecordAnalysisTask(taskType string, success bool, duration float64) {}
func (n *NoopAnalyzerMetrics) RecordLinkVerification(success bool, duration float64) {
}
func (n *NoopAnalyzerMetrics) RecordHTTPClientRequest(statusCode int, duration float64, method, requestType string) {
}
func (n *NoopAnalyzerMetrics) SetConcurrentLinkVerifications(count int) {}

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

func (m *AnalyzerMetrics) RecordAnalysisJob(success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}

	m.AnalysisJobsProcessedTotal.WithLabelValues(status).Inc()
	m.AnalysisDuration.WithLabelValues().Observe(duration)
}

func (m *AnalyzerMetrics) RecordAnalysisTask(taskType string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "error"
	}
	m.AnalysisTasksCompletedTotal.WithLabelValues(taskType, status).Inc()
	m.AnalysisTaskDuration.WithLabelValues(taskType).Observe(duration)
}

func (m *AnalyzerMetrics) RecordLinkVerification(success bool, duration float64) {
	outcome := "success"
	if !success {
		outcome = "failed"
	}

	m.LinksVerifiedTotal.WithLabelValues(outcome).Inc()
	m.LinkVerificationDuration.WithLabelValues(outcome).Observe(duration)
}

func (m *AnalyzerMetrics) RecordHTTPClientRequest(status int, duration float64, method, requestType string) {
	m.HTTPClientRequestsTotal.WithLabelValues(strconv.Itoa(status), method, requestType).Inc()
	m.HTTPClientRequestDuration.WithLabelValues(method, requestType).Observe(duration)
}

func (m *AnalyzerMetrics) SetConcurrentLinkVerifications(count int) {
	m.ConcurrentLinkVerifications.Set(float64(count))
}
