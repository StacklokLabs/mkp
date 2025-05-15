package mcp

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/StacklokLabs/mkp/pkg/ratelimit"
)

// WithTimeoutContext adds a timeout context to all tool handlers
// This helps prevent context cancellation errors from the k8s client
func WithTimeoutContext(timeout time.Duration) server.ServerOption {
	return server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
			// Extract session ID from the original context
			var sessionID string
			if session := server.ClientSessionFromContext(ctx); session != nil {
				sessionID = session.SessionID()
			}

			// Create a fresh context with a longer timeout (prevents cancellation)
			timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Add the session ID to the new context if we found one
			if sessionID != "" {
				timeoutCtx = ratelimit.SetSessionIDToContext(timeoutCtx, sessionID)
			}

			// Call the next handler with the timeout context
			return next(timeoutCtx, request)
		}
	})
}
