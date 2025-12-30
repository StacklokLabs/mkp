// Package otel provides OpenTelemetry instrumentation for the MCP server
package otel

import (
	"os"
)

// Config holds the OpenTelemetry configuration
type Config struct {
	// Enabled determines whether OpenTelemetry is enabled
	Enabled bool
	// ServiceName is the name of the service for tracing
	ServiceName string
	// ServiceVersion is the version of the service
	ServiceVersion string
	// OTLPEndpoint is the endpoint for the OTLP exporter (e.g., "localhost:4317")
	// If empty, stdout exporter is used for debugging
	OTLPEndpoint string
}

// DefaultConfig returns the default OpenTelemetry configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:        getEnvBool("MKP_OTEL_ENABLED", false),
		ServiceName:    getEnvString("MKP_OTEL_SERVICE_NAME", "mkp"),
		ServiceVersion: getEnvString("MKP_OTEL_SERVICE_VERSION", "0.1.0"),
		OTLPEndpoint:   getEnvString("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getEnvString(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
