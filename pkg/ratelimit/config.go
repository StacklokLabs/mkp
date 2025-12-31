// Package ratelimit provides rate limiting functionality for the MCP server
package ratelimit

import (
	"os"
	"strconv"

	"github.com/StacklokLabs/mkp/pkg/types"
)

const (
	// DefaultTool is the key for the default rate limit
	DefaultTool = "default"

	// Default values for rate limits (requests per minute)
	defaultDefaultLimit = 60
	defaultReadLimit    = 120 // 120 requests per minute (2 per second)
	defaultWriteLimit   = 30  // 30 requests per minute (0.5 per second)
)

// Config holds the rate limiting configuration
type Config struct {
	// DefaultLimit is the default rate limit for tools not explicitly configured
	DefaultLimit int
	// ReadLimit is the rate limit for read operations (list_resources, get_resource)
	ReadLimit int
	// WriteLimit is the rate limit for write operations (apply_resource, delete_resource)
	WriteLimit int
}

// NewDefaultConfig returns a Config with default values, optionally overridden by environment variables:
//   - MKP_RATE_LIMIT_DEFAULT: default rate limit (default: 60)
//   - MKP_RATE_LIMIT_READ: read operations rate limit (default: 120)
//   - MKP_RATE_LIMIT_WRITE: write operations rate limit (default: 30)
func NewDefaultConfig() *Config {
	return &Config{
		DefaultLimit: getEnvInt("MKP_RATE_LIMIT_DEFAULT", defaultDefaultLimit),
		ReadLimit:    getEnvInt("MKP_RATE_LIMIT_READ", defaultReadLimit),
		WriteLimit:   getEnvInt("MKP_RATE_LIMIT_WRITE", defaultWriteLimit),
	}
}

// getEnvInt returns the integer value of an environment variable or a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil && intValue > 0 {
			return intValue
		}
	}
	return defaultValue
}

// GetDefaultRateLimiter returns a RateLimiter with default configuration
func GetDefaultRateLimiter() *RateLimiter {
	return GetRateLimiterWithConfig(nil)
}

// GetRateLimiterWithConfig returns a RateLimiter with the given configuration.
// If config is nil, default configuration is used.
func GetRateLimiterWithConfig(config *Config) *RateLimiter {
	if config == nil {
		config = NewDefaultConfig()
	}

	options := []RateLimiterOption{
		WithDefaultLimit(config.DefaultLimit),
		WithToolLimit(types.ListResourcesToolName, config.ReadLimit),
		WithToolLimit(types.GetResourceToolName, config.ReadLimit),
		WithToolLimit(types.ApplyResourceToolName, config.WriteLimit),
		WithToolLimit(types.DeleteResourceToolName, config.WriteLimit),
	}

	return NewRateLimiter(options...)
}
