package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	apiServiceName = "api"
)

// APIMetrics is a struct for API metrics
type APIMetrics struct {
	*ServiceMetrics

	JobsCreatedTotal    *prometheus.CounterVec
	JobCreationDuration *prometheus.HistogramVec
}

// NewAPIMetrics creates a new API metrics
func NewAPIMetrics() *APIMetrics {
	baseMetrics := NewServiceMetrics(apiServiceName)

	apiMetrics := &APIMetrics{
		ServiceMetrics: baseMetrics,

		JobsCreatedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "jobs_created_total",
				Help:        "Total number of analysis jobs created",
				ConstLabels: prometheus.Labels{LabelService: apiServiceName},
			},
			[]string{LabelStatus},
		),

		JobCreationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "job_creation_duration_seconds",
				Help:        "Time taken to create a job in seconds",
				Buckets:     prometheus.DefBuckets,
				ConstLabels: prometheus.Labels{LabelService: apiServiceName},
			},
			[]string{},
		),
	}

	return apiMetrics
}

// MustRegisterAPI registers the API metrics and base service metrics
func (m *APIMetrics) MustRegisterAPI() {
	m.ServiceMetrics.MustRegister()

	prometheus.MustRegister(
		m.JobsCreatedTotal,
		m.JobCreationDuration,
	)
}

// RecordJobCreation records the job creation metrics
func (m *APIMetrics) RecordJobCreation(success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "error"
	}
	m.JobsCreatedTotal.WithLabelValues(status).Inc()
	m.JobCreationDuration.WithLabelValues().Observe(duration.Seconds())
}
