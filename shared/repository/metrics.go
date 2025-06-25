package repository

import (
	"time"
)

type MetricsCollector interface {
	RecordDatabaseOperation(operation, table string, start time.Time, err error)
}

type NoOpMetricsCollector struct{}

func (n NoOpMetricsCollector) RecordDatabaseOperation(operation, table string, start time.Time, err error) {
}
