package config

import (
	"shared/config"
)

// Config holds all configuration for the analyzer service
type Config struct {
	Service  config.ServiceConfig
	HTTP     config.HTTPClientConfig
	Metrics  config.MetricsConfig
	Tracing  config.TracingConfig
	Database config.DatabaseConfig
	NATS     config.NATSConfig
}

// Load loads the configuration for the analyzer service
func Load() *Config {
	return &Config{
		Service:  config.NewServiceConfig("analyzer"),
		HTTP:     config.NewHTTPClientConfig(),
		Metrics:  config.NewMetricsConfig("9091"),
		Tracing:  config.NewTracingConfig("analyzer"),
		Database: config.DatabaseConfig{},
		NATS:     config.NewNATSConfig(),
	}
}
