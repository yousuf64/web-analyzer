package config

import (
	"os"
	"strconv"
	"time"
)

// Common configuration types used across all services

// ServiceConfig holds basic service information
type ServiceConfig struct {
	Name    string
	Version string
	Port    string
}

// MetricsConfig holds metrics server configuration
type MetricsConfig struct {
	Port string
}

// NATSConfig holds NATS connection configuration
type NATSConfig struct {
	URL string
}

// TracingConfig holds tracing configuration
type TracingConfig struct {
	ServiceName string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct{}

// HTTPServerConfig holds HTTP server configuration
type HTTPServerConfig struct {
	Addr string
	Port string
}

// HTTPClientConfig holds HTTP client configuration
type HTTPClientConfig struct {
	Timeout       time.Duration
	MaxConcurrent int
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	MaxConnections int
	ReadTimeout    int // seconds
	WriteTimeout   int // seconds
}

// Common environment variable parsing functions

// GetEnv gets an environment variable with a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetIntEnv gets an integer environment variable with a default value
func GetIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetDurationEnv gets a duration environment variable with a default value
func GetDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetBoolEnv gets a boolean environment variable with a default value
func GetBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// Common configuration builders

// NewServiceConfig creates a ServiceConfig with common defaults
func NewServiceConfig(serviceName string) ServiceConfig {
	return ServiceConfig{
		Name:    GetEnv("SERVICE_NAME", serviceName),
		Version: GetEnv("SERVICE_VERSION", "1.0.0"),
		Port:    GetEnv("PORT", "8080"),
	}
}

// NewMetricsConfig creates a MetricsConfig with common defaults
func NewMetricsConfig(defaultPort string) MetricsConfig {
	return MetricsConfig{
		Port: GetEnv("METRICS_PORT", defaultPort),
	}
}

// NewNATSConfig creates a NATSConfig with common defaults
func NewNATSConfig() NATSConfig {
	return NATSConfig{
		URL: GetEnv("NATS_URL", "nats://localhost:4222"),
	}
}

// NewTracingConfig creates a TracingConfig with common defaults
func NewTracingConfig(serviceName string) TracingConfig {
	return TracingConfig{
		ServiceName: GetEnv("TRACING_SERVICE_NAME", serviceName),
	}
}

// NewHTTPServerConfig creates an HTTPServerConfig with common defaults
func NewHTTPServerConfig(defaultAddr, defaultPort string) HTTPServerConfig {
	return HTTPServerConfig{
		Addr: GetEnv("HTTP_ADDR", defaultAddr),
		Port: GetEnv("HTTP_PORT", defaultPort),
	}
}

// NewHTTPClientConfig creates an HTTPClientConfig with common defaults
func NewHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:       GetDurationEnv("HTTP_CLIENT_TIMEOUT", 20*time.Second),
		MaxConcurrent: GetIntEnv("HTTP_MAX_CONCURRENT", 10),
	}
}

// NewWebSocketConfig creates a WebSocketConfig with common defaults
func NewWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		MaxConnections: GetIntEnv("WS_MAX_CONNECTIONS", 1000),
		ReadTimeout:    GetIntEnv("WS_READ_TIMEOUT", 60),
		WriteTimeout:   GetIntEnv("WS_WRITE_TIMEOUT", 10),
	}
}
