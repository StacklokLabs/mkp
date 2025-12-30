package otel

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware(t *testing.T) {
	middleware := Middleware()
	assert.NotNil(t, middleware)

	// Create a mock handler
	mockHandler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("success"), nil
	}

	wrappedHandler := middleware(mockHandler)
	assert.NotNil(t, wrappedHandler)

	request := mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name: "test-tool",
		},
	}

	result, err := wrappedHandler(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestMiddlewareWithError(t *testing.T) {
	middleware := Middleware()

	// Create a handler that returns an error result
	errorHandler := func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError("test error"), nil
	}

	wrappedHandler := middleware(errorHandler)

	request := mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Name: "test-tool",
		},
	}

	result, err := wrappedHandler(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}
