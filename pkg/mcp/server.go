package mcp

import (
	"context"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/ratelimit"
)

// defaultCtxTimeout is the default timeout for tool calls
const defaultCtxTimeout = 30 * time.Second

// Config holds configuration options for the MCP server
type Config struct {
	// ServeResources determines whether to serve cluster resources
	// Setting this to false can reduce context size for LLMs when working with large clusters
	ServeResources bool

	// ReadWrite determines whether the MCP server can modify resources in the cluster
	// When false, the server operates in read-only mode and does not serve the apply_resource tool
	ReadWrite bool

	// EnableRateLimiting determines whether to enable rate limiting for tool calls
	// When true, a default rate limiter will be used to prevent excessive API calls
	EnableRateLimiting bool
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		ServeResources:     true,  // Default to serving resources for backward compatibility
		ReadWrite:          false, // Default to read-only mode
		EnableRateLimiting: true,  // Default to enabling rate limiting
	}
}

// serverResources holds resources that need to be cleaned up when the server is stopped
type serverResources struct {
	rateLimiter *ratelimit.RateLimiter
}

// Global variable to hold server resources
var resources *serverResources

// CreateServer creates a new MCP server for Kubernetes
func CreateServer(k8sClient *k8s.Client, config *Config) *server.MCPServer {
	// Use default config if none provided
	if config == nil {
		config = DefaultConfig()
	}
	// Create MCP implementation
	impl := NewImplementation(k8sClient)

	options := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		// Add timeout middleware to prevent context cancellation errors
		WithTimeoutContext(defaultCtxTimeout),
		server.WithRecovery(),
	}

	// Add rate limiting middleware if enabled
	if config.EnableRateLimiting {
		// Create and store the rate limiter for later cleanup
		limiter := ratelimit.GetDefaultRateLimiter()

		// Store the limiter for cleanup when the server is stopped
		resources = &serverResources{
			rateLimiter: limiter,
		}

		options = append(options, server.WithToolHandlerMiddleware(limiter.Middleware()))
	}

	// Create MCP server with all options
	mcpServer := server.NewMCPServer(
		"kubernetes-mcp-server",
		"0.1.0",
		options...,
	)

	// Add tools
	mcpServer.AddTool(NewListResourcesTool(), impl.HandleListResources)
	mcpServer.AddTool(NewGetResourceTool(), impl.HandleGetResource)

	if config.ReadWrite {
		mcpServer.AddTool(NewApplyResourceTool(), impl.HandleApplyResource)
		mcpServer.AddTool(NewDeleteResourceTool(), impl.HandleDeleteResource)
	}

	// Add resource templates
	mcpServer.AddResourceTemplate(
		NewClusteredResourceTemplate(),
		impl.HandleClusteredResource,
	)
	mcpServer.AddResourceTemplate(
		NewNamespacedResourceTemplate(),
		impl.HandleNamespacedResource,
	)

	// Add resources if enabled
	if config.ServeResources {
		go func() {
			// Create a timeout context for listing resources
			timeoutCtx, cancel := context.WithTimeout(context.Background(), defaultCtxTimeout)
			defer cancel()

			// List resources in a goroutine to avoid blocking server startup
			resources, err := impl.HandleListAllResources(timeoutCtx)
			if err != nil {
				log.Printf("Failed to list resources: %v", err)
				return
			}

			// Add resources to the server
			for _, resource := range resources {
				mcpServer.AddResource(resource, nil)
			}
		}()
	}

	return mcpServer
}

// StopServer stops the MCP server and cleans up resources
func StopServer() {
	// Clean up resources
	if resources != nil {
		// Stop the rate limiter if it exists
		if resources.rateLimiter != nil {
			resources.rateLimiter.Stop()
		}
	}
}

// CreateSSEServer creates a new SSE server for the MCP server
func CreateSSEServer(mcpServer *server.MCPServer) *server.SSEServer {
	return server.NewSSEServer(mcpServer)
}
