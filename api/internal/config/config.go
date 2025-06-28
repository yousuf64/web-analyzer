package config

import (
	"shared/config"
)

// Config holds all configuration for the API service
type Config struct {
	Service config.ServiceConfig
	HTTP    config.HTTPServerConfig
	Metrics config.MetricsConfig
	NATS    config.NATSConfig
}

// Load loads the configuration for the API service
func Load() *Config {
	return &Config{
		Service: config.NewServiceConfig("api"),
		HTTP:    config.NewHTTPServerConfig(":8080", "8080"),
		Metrics: config.NewMetricsConfig("9090"),
		NATS:    config.NewNATSConfig(),
	}
}
