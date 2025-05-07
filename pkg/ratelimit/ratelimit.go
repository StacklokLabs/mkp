package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	cleanupInterval = 10 * time.Minute
	bucketTimeout   = 30 * time.Minute
)

// RateLimiter implements a rate limiting middleware for MCP server
// It uses a token bucket algorithm to limit the number of requests per minute for each tool
type RateLimiter struct {
	mu            sync.RWMutex
	limits        map[string]int                // Tool name to requests per minute
	defaultLimit  int                           // Default requests per minute
	buckets       map[string]map[string]*bucket // SessionID:[Tool:Bucket] mapping
	cleanupTicker *time.Ticker
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	mu       sync.Mutex
	tokens   int       // Current number of tokens
	lastSeen time.Time // Last time this bucket was accessed
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

// NewRateLimiter creates a new rate limiter with the given options
func NewRateLimiter(opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		limits:       make(map[string]int),
		defaultLimit: defaultLimit,
		buckets:      make(map[string]map[string]*bucket),
	}

	for _, opt := range opts {
		opt(rl)
	}

	// Start a cleanup ticker to remove old buckets
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
	for sessionID, toolBuckets := range rl.buckets {
		for tool, b := range toolBuckets {
			b.mu.Lock()
			// If bucket hasn't been used for bucketTimeout, remove it
			if now.Sub(b.lastSeen) > bucketTimeout {
				delete(toolBuckets, tool)
			}
			b.mu.Unlock()
		}
		// If no more buckets for this session, remove the session entry
		if len(toolBuckets) == 0 {
			delete(rl.buckets, sessionID)
		}
	}
}

// Stop stops the cleanup ticker
func (rl *RateLimiter) Stop() {
	if rl.cleanupTicker != nil {
		rl.cleanupTicker.Stop()
	}
}

// getSessionID extracts the session ID from the request context
func getSessionID(ctx context.Context) string {
	// Get the session from the context
	if session := server.ClientSessionFromContext(ctx); session != nil {
		return session.SessionID()
	}
	// If no session is available (which shouldn't happen in normal operation),
	// return a default identifier
	return "unknown"
}

// getBucket gets or creates a bucket for the given session ID and tool
func (rl *RateLimiter) getBucket(sessionID, tool string) *bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Create session map if it doesn't exist
	if _, ok := rl.buckets[sessionID]; !ok {
		rl.buckets[sessionID] = make(map[string]*bucket)
	}

	// Create bucket if it doesn't exist
	if _, ok := rl.buckets[sessionID][tool]; !ok {
		rl.buckets[sessionID][tool] = &bucket{
			tokens:   rl.getLimit(tool), // Initialize with full tokens
			lastSeen: time.Now(),
		}
	}

	return rl.buckets[sessionID][tool]
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
			sessionID := getSessionID(ctx)
			tool := request.Params.Name

			b := rl.getBucket(sessionID, tool)
			b.mu.Lock()
			defer b.mu.Unlock()

			now := time.Now()
			b.lastSeen = now

			// Calculate tokens to add based on time elapsed
			limit := rl.getLimit(tool)
			tokensPerSecond := float64(limit) / 60.0
			elapsed := now.Sub(b.lastSeen).Seconds()
			tokensToAdd := int(elapsed * tokensPerSecond)

			// Add tokens, but don't exceed the limit
			b.tokens = min(b.tokens+tokensToAdd, limit)

			// Check if we have enough tokens
			if b.tokens <= 0 {
				return mcp.NewToolResultError(fmt.Sprintf("Rate limit exceeded for tool '%s'. Try again later.", tool)), nil
			}

			// Consume a token
			b.tokens--

			// Call the next handler
			return next(ctx, request)
		}
	}
}
