package config

import (
	"shared/config"
)

// Config is the configuration for the notifications service
type Config struct {
	Service   config.ServiceConfig
	HTTP      config.HTTPServerConfig
	WebSocket config.WebSocketConfig
	Metrics   config.MetricsConfig
	Tracing   config.TracingConfig
	NATS      config.NATSConfig
}

// Load loads the configuration for the notifications service
func Load() *Config {
	return &Config{
		Service:   config.NewServiceConfig("notifications"),
		HTTP:      config.NewHTTPServerConfig(":8081"),
		WebSocket: config.NewWebSocketConfig(),
		Metrics:   config.NewMetricsConfig("9092"),
		Tracing:   config.NewTracingConfig("notifications"),
		NATS:      config.NewNATSConfig(),
	}
}
