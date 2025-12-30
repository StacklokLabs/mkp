package otel

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, "mkp", config.ServiceName)
	assert.Equal(t, "0.1.0", config.ServiceVersion)
	assert.Empty(t, config.OTLPEndpoint)
}

func TestConfigFromEnv(t *testing.T) {
	// Save original env values
	origEnabled := os.Getenv("MKP_OTEL_ENABLED")
	origName := os.Getenv("MKP_OTEL_SERVICE_NAME")
	origVersion := os.Getenv("MKP_OTEL_SERVICE_VERSION")
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	defer func() {
		os.Setenv("MKP_OTEL_ENABLED", origEnabled)
		os.Setenv("MKP_OTEL_SERVICE_NAME", origName)
		os.Setenv("MKP_OTEL_SERVICE_VERSION", origVersion)
		os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
	}()

	// Set custom env values
	os.Setenv("MKP_OTEL_ENABLED", "true")
	os.Setenv("MKP_OTEL_SERVICE_NAME", "test-service")
	os.Setenv("MKP_OTEL_SERVICE_VERSION", "1.0.0")
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

	config := DefaultConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, "test-service", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "localhost:4317", config.OTLPEndpoint)
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"false", "false", false},
		{"0", "0", false},
		{"empty", "", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := "TEST_BOOL_" + tt.name
			defer os.Unsetenv(key)

			if tt.envValue != "" {
				os.Setenv(key, tt.envValue)
			}
			result := getEnvBool(key, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}
