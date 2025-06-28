package config

import (
	"shared/config"
)

// Config holds all configuration for the API service
type Config struct {
	Service  config.ServiceConfig
	HTTP     config.HTTPServerConfig
	Metrics  config.MetricsConfig
	Tracing  config.TracingConfig
	DynamoDB config.DynamoDBConfig
	NATS     config.NATSConfig
}

// Load loads the configuration for the API service
func Load() *Config {
	return &Config{
		Service:  config.NewServiceConfig("api"),
		HTTP:     config.NewHTTPServerConfig(":8080"),
		Metrics:  config.NewMetricsConfig("9090"),
		Tracing:  config.NewTracingConfig("api"),
		DynamoDB: config.NewDynamoDBConfig(),
		NATS:     config.NewNATSConfig(),
	}
}
