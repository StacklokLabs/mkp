package ratelimit

const defaultLimit = 60

// DefaultConfig defines the default rate limits for different tools
var DefaultConfig = map[string]int{
	// Read operations - higher limits
	"list_resources": 120, // 120 requests per minute (2 per second)
	"get_resource":   120, // 120 requests per minute (2 per second)
	"read_resource":  120, // 120 requests per minute (2 per second)

	// Write operations - lower limits
	"apply_resource":  30, // 30 requests per minute (0.5 per second)
	"delete_resource": 30, // 30 requests per minute (0.5 per second)

	// Default for any other tool
	"default": defaultLimit,
}

// GetDefaultRateLimiter returns a RateLimiter with default configuration
func GetDefaultRateLimiter() *RateLimiter {
	options := []RateLimiterOption{
		WithDefaultLimit(DefaultConfig["default"]),
	}

	// Add tool-specific limits
	for tool, limit := range DefaultConfig {
		if tool != "default" {
			options = append(options, WithToolLimit(tool, limit))
		}
	}

	return NewRateLimiter(options...)
}
