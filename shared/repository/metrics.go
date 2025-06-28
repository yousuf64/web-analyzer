package repository

import (
	"time"
)

// MetricsCollector is an interface for recording database metrics
type MetricsCollector interface {
	RecordDatabaseOperation(operation, table string, start time.Time, err error)
}

// NoOpMetricsCollector is a no-op implementation of MetricsCollector
type NoOpMetricsCollector struct{}

// RecordDatabaseOperation is a no-op implementation of RecordDatabaseOperation
func (n NoOpMetricsCollector) RecordDatabaseOperation(operation, table string, start time.Time, err error) {
}
