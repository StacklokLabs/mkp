package ratelimit

import "github.com/StacklokLabs/mkp/pkg/types"

// TODO: make these constants configurable
const (
	defaultLimit = 60
	readLimit    = 120 // 120 requests per minute (2 per second)
	writeLimit   = 30  // 30 requests per minute (0.5 per second)
	DefaultTool  = "default"
)

// DefaultConfig defines the default rate limits for different tools
var DefaultConfig = map[string]int{
	// Read operations - higher limits
	types.ListResourcesToolName: readLimit,
	types.GetResourceToolName:   readLimit,

	// Write operations - lower limits
	types.ApplyResourceToolName:  writeLimit,
	types.DeleteResourceToolName: writeLimit,

	// Default for any other tool
	DefaultTool: defaultLimit,
}

// GetDefaultRateLimiter returns a RateLimiter with default configuration
func GetDefaultRateLimiter() *RateLimiter {
	options := []RateLimiterOption{
		WithDefaultLimit(DefaultConfig[DefaultTool]),
	}

	// Add tool-specific limits
	for tool, limit := range DefaultConfig {
		if tool != DefaultTool {
			options = append(options, WithToolLimit(tool, limit))
		}
	}

	limiter := NewRateLimiter(options...)
	return limiter
}
