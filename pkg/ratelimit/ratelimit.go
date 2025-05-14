package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	cleanupInterval = 10 * time.Minute
	windowSize      = 60 * time.Second
)

// RateLimiter implements a rate limiting middleware for MCP server
// Using a fixed window rate limiting algorithm
type RateLimiter struct {
	mu            sync.RWMutex
	limits        map[string]int                       // Tool name to requests per minute
	defaultLimit  int                                  // Default requests per minute
	requestCounts map[string]map[string]*windowCounter // SessionID:[Tool:Counter] mapping
	cleanupTicker *time.Ticker
}

// windowCounter tracks requests in the current time window
type windowCounter struct {
	mu          sync.Mutex
	count       int       // Number of requests in the current window
	windowStart time.Time // Start time of the current window
}

// RateLimiterOption is a function that configures a RateLimiter
type RateLimiterOption func(*RateLimiter)

// WithToolLimit sets the rate limit for a specific tool
func WithToolLimit(toolName string, requestsPerMinute int) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.limits[toolName] = requestsPerMinute
	}
}

// WithDefaultLimit sets the default rate limit for all tools
func WithDefaultLimit(requestsPerMinute int) RateLimiterOption {
	return func(rl *RateLimiter) {
		rl.defaultLimit = requestsPerMinute
	}
}

// WithTimeWindow sets the time window for rate limiting
func WithTimeWindow(window time.Duration) RateLimiterOption {
	return func(_ *RateLimiter) {
		windowSize = window
	}
}

// NewRateLimiter creates a new rate limiter with the given options
func NewRateLimiter(opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		limits:        make(map[string]int),
		defaultLimit:  defaultLimit,
		requestCounts: make(map[string]map[string]*windowCounter),
	}

	for _, opt := range opts {
		opt(rl)
	}

	// Start a cleanup ticker to remove old counters
	rl.cleanupTicker = time.NewTicker(cleanupInterval)
	go func() {
		for range rl.cleanupTicker.C {
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for sessionID, toolCounters := range rl.requestCounts {
		for tool, counter := range toolCounters {
			counter.mu.Lock()
			// If window hasn't been updated for more than cleanup interval, remove it
			if now.Sub(counter.windowStart) > cleanupInterval {
				delete(toolCounters, tool)
			}
			counter.mu.Unlock()
		}
		// If no more counters for this session, remove the session entry
		if len(toolCounters) == 0 {
			delete(rl.requestCounts, sessionID)
		}
	}
}

// Stop stops the cleanup ticker
func (rl *RateLimiter) Stop() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}

// Custom context key for session IDs
type sessionIDKey struct{}

// SetSessionIDToContext adds the session ID to a context
func SetSessionIDToContext(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// getSessionID extracts the session ID from the request context, or falls back to tool name
func getSessionID(ctx context.Context, toolName string) string {
	// First check if we have a session ID directly in the context via our custom key
	if sessionID, ok := ctx.Value(sessionIDKey{}).(string); ok && sessionID != "" {
		return sessionID
	}

	// Try to get the session from the context via server's method
	if session := server.ClientSessionFromContext(ctx); session != nil {
		sessionID := session.SessionID()
		return sessionID
	}

	// If not found in either case: use the tool name as part of the identifier
	return "tool:" + toolName
}

// getCounter gets or creates a request counter for the given session ID and tool
func (rl *RateLimiter) getCounter(sessionID, tool string) *windowCounter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Create session map if it doesn't exist
	if _, ok := rl.requestCounts[sessionID]; !ok {
		rl.requestCounts[sessionID] = make(map[string]*windowCounter)
	}

	// Create counter if it doesn't exist
	if _, ok := rl.requestCounts[sessionID][tool]; !ok {
		rl.requestCounts[sessionID][tool] = &windowCounter{
			count:       0,
			windowStart: time.Now(),
		}
	}

	return rl.requestCounts[sessionID][tool]
}

// getLimit returns the rate limit for the given tool
func (rl *RateLimiter) getLimit(tool string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if limit, ok := rl.limits[tool]; ok {
		return limit
	}
	return rl.defaultLimit
}

// Middleware returns a middleware function for the MCP server
func (rl *RateLimiter) Middleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tool := request.Params.Name

			// Get session ID or use tool-based identifier if session not available
			sessionID := getSessionID(ctx, tool)

			// Get the request counter for this session and tool
			counter := rl.getCounter(sessionID, tool)
			counter.mu.Lock()
			defer counter.mu.Unlock()

			// Get current time
			now := time.Now()

			// Check if we need to reset the window
			if now.Sub(counter.windowStart) >= windowSize {
				counter.count = 0
				counter.windowStart = now
			}

			// Get the limit for this tool
			limit := rl.getLimit(tool)

			// Check if we've exceeded the limit
			if counter.count >= limit {
				timeToNextWindow := windowSize - now.Sub(counter.windowStart)
				return mcp.NewToolResultError(fmt.Sprintf(
					"Rate limit exceeded for tool '%s'. Try again in %.1f seconds.",
					tool, timeToNextWindow.Seconds())), nil
			}

			// Increment the counter
			counter.count++

			// Call the next handler
			return next(ctx, request)
		}
	}
}
