package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRateLimiterCreation(t *testing.T) {
	// Test with default options
	limiter := NewRateLimiter()
	assert.NotNil(t, limiter)
	assert.Equal(t, defaultLimit, limiter.defaultLimit)

	// Test with custom options
	customLimiter := NewRateLimiter(
		WithDefaultLimit(100),
		WithToolLimit("test-tool", 50),
	)
	assert.NotNil(t, customLimiter)
	assert.Equal(t, 100, customLimiter.defaultLimit)
	assert.Equal(t, 50, customLimiter.limits["test-tool"])
}

func TestGetSessionID(t *testing.T) {
	// Test with session ID in context via custom key
	ctx := context.Background()
	ctx = SetSessionIDToContext(ctx, "test-session")
	sessionID := getSessionID(ctx, "test-tool")
	assert.Equal(t, "test-session", sessionID)

	// Test with ClientSession in context
	ctx = context.Background()
	session := &mockSession{id: "mock-session"}
	ctx = SetSessionIDToContext(ctx, session.SessionID())
	sessionID = getSessionID(ctx, "test-tool")
	assert.Equal(t, "mock-session", sessionID)

	// Test fallback to tool name
	ctx = context.Background()
	sessionID = getSessionID(ctx, "fallback-tool")
	assert.Equal(t, "tool:fallback-tool", sessionID)
}

func TestRateLimiting(t *testing.T) {
	// Rate limiter with a low limit for testing
	limiter := NewRateLimiter(WithDefaultLimit(2))
	middleware := limiter.Middleware()

	// Mock handler that always succeeds
	mockHandler := new(MockToolHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(
		mcp.NewToolResultText("success"), nil,
	)

	wrappedHandler := middleware(mockHandler.Handle)

	request := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "test-tool",
		},
	}

	ctx := SetSessionIDToContext(context.Background(), "test-session")

	// First request should succeed
	result, err := wrappedHandler(ctx, request)
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	// Second request should succeed (hit limit)
	result, err = wrappedHandler(ctx, request)
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	// Third request should be rate limited
	result, err = wrappedHandler(ctx, request)
	assert.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")
}

func TestMultipleSessionRateLimiting(t *testing.T) {
	limiter := NewRateLimiter(WithDefaultLimit(2))
	middleware := limiter.Middleware()

	mockHandler := new(MockToolHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(
		mcp.NewToolResultText("success"), nil,
	)

	wrappedHandler := middleware(mockHandler.Handle)

	request := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "test-tool",
		},
	}

	// Create contexts for two different sessions
	ctx1 := SetSessionIDToContext(context.Background(), "session-1")
	ctx2 := SetSessionIDToContext(context.Background(), "session-2")

	// Session 1: First request
	result, _ := wrappedHandler(ctx1, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Session 1: Second request (hits limit)
	result, _ = wrappedHandler(ctx1, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Session 1: Third request (rate limited)
	result, _ = wrappedHandler(ctx1, request)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")

	// Session 2: First request (should succeed because different session)
	result, _ = wrappedHandler(ctx2, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Session 2: Second request (hits limit)
	result, _ = wrappedHandler(ctx2, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Session 2: Third request (rate limited)
	result, _ = wrappedHandler(ctx2, request)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")
}

func TestWindowReset(t *testing.T) {
	// Rate limiter with a small window for testing
	testWindowSize := 50 * time.Millisecond

	// Limiter with custom window size
	limiter := NewRateLimiter(WithDefaultLimit(1), WithTimeWindow(testWindowSize))
	middleware := limiter.Middleware()

	mockHandler := new(MockToolHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(
		mcp.NewToolResultText("success"), nil,
	)

	wrappedHandler := middleware(mockHandler.Handle)

	request := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "test-tool",
		},
	}

	ctx := SetSessionIDToContext(context.Background(), "test-session")

	// First request should succeed
	result, _ := wrappedHandler(ctx, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Second request should be rate limited
	result, _ = wrappedHandler(ctx, request)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")

	// Wait for the window to reset
	time.Sleep(testWindowSize + 10*time.Millisecond)

	// After window reset, the request should succeed again
	result, _ = wrappedHandler(ctx, request)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)
}

func TestToolSpecificLimits(t *testing.T) {
	// Rate limiter with tool-specific limits
	limiter := NewRateLimiter(
		WithDefaultLimit(1),
		WithToolLimit("high-limit-tool", 3),
	)
	middleware := limiter.Middleware()

	mockHandler := new(MockToolHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(
		mcp.NewToolResultText("success"), nil,
	)
	wrappedHandler := middleware(mockHandler.Handle)

	ctx := SetSessionIDToContext(context.Background(), "test-session")

	// Test default limit tool (limit = 1)
	defaultRequest := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "default-tool",
		},
	}

	// First request should succeed
	result, _ := wrappedHandler(ctx, defaultRequest)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Second request should be rate limited
	result, _ = wrappedHandler(ctx, defaultRequest)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")

	// Test high limit tool (limit = 3)
	highLimitRequest := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name: "high-limit-tool",
		},
	}

	// First request should succeed
	result, _ = wrappedHandler(ctx, highLimitRequest)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Second request should succeed
	result, _ = wrappedHandler(ctx, highLimitRequest)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Third request should succeed
	result, _ = wrappedHandler(ctx, highLimitRequest)
	assert.False(t, result.IsError)
	assert.Equal(t, "success", result.Content[0].(mcp.TextContent).Text)

	// Fourth request should be rate limited
	result, _ = wrappedHandler(ctx, highLimitRequest)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Rate limit exceeded")
}
