package mcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// WithTimeoutContext adds a timeout context to all tool handlers
// This helps prevent context cancellation errors from the k8s client
func WithTimeoutContext(timeout time.Duration) server.ServerOption {
	return server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(_ context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			// Create a new context with the specified timeout
			timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Call the next handler with the timeout context
			return next(timeoutCtx, request)
		}
	})
}
