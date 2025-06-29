package messagebus

import "time"

type MetricsCollector interface {
	RecordNATSPublish(messageType string, success bool)
	RecordNATSReceive(messageType string, duration time.Duration, success bool)
}

type NoOpMetricsCollector struct{}

func (n NoOpMetricsCollector) RecordNATSPublish(messageType string, success bool) {}
func (n NoOpMetricsCollector) RecordNATSReceive(messageType string, duration time.Duration, success bool) {
}
