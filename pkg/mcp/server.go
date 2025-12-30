package mcp

import (
	"context"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/otel"
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

	// EnableOtel determines whether to enable OpenTelemetry tracing and metrics
	// When true, tool calls will be instrumented with OpenTelemetry
	EnableOtel bool
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	otelConfig := otel.DefaultConfig()
	return &Config{
		ServeResources:     true,           // Default to serving resources for backward compatibility
		ReadWrite:          false,          // Default to read-only mode
		EnableRateLimiting: true,           // Default to enabling rate limiting
		EnableOtel:         otelConfig.Enabled, // Default based on MKP_OTEL_ENABLED env var
	}
}

// serverResources holds resources that need to be cleaned up when the server is stopped
type serverResources struct {
	rateLimiter  *ratelimit.RateLimiter
	otelProvider *otel.Provider
}

// Global variable to hold server resources
var resources *serverResources

// CreateServer creates a new MCP server for Kubernetes
func CreateServer(k8sClient *k8s.Client, config *Config) *server.MCPServer {
	// Use default config if none provided
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize server resources
	resources = &serverResources{}

	// Create MCP implementation
	impl := NewImplementation(k8sClient)

	options := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		// Add timeout middleware to prevent context cancellation errors
		WithTimeoutContext(defaultCtxTimeout),
		server.WithRecovery(),
	}

	// Add OpenTelemetry middleware if enabled
	if config.EnableOtel {
		log.Println("OpenTelemetry enabled, initializing provider")
		otelConfig := otel.DefaultConfig()
		provider, err := otel.NewProvider(context.Background(), otelConfig)
		if err != nil {
			log.Printf("Failed to initialize OpenTelemetry: %v", err)
		} else {
			resources.otelProvider = provider
			options = append(options, server.WithToolHandlerMiddleware(otel.Middleware()))
		}
	}

	// Add rate limiting middleware if enabled
	if config.EnableRateLimiting {
		log.Println("Server rate limiting enabled, initializing rate limiter")
		limiter := ratelimit.GetDefaultRateLimiter()
		resources.rateLimiter = limiter
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
		mcpServer.AddTool(NewPostResourceTool(), impl.HandlePostResource)
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
	if resources == nil {
		return
	}

	// Stop the rate limiter if it exists
	if resources.rateLimiter != nil {
		resources.rateLimiter.Stop()
	}

	// Shutdown OpenTelemetry provider if it exists
	if resources.otelProvider != nil {
		if err := resources.otelProvider.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down OpenTelemetry: %v", err)
		}
	}
}

// CreateSSEServer creates a new SSE server for the MCP server
func CreateSSEServer(mcpServer *server.MCPServer) *server.SSEServer {
	return server.NewSSEServer(mcpServer)
}

// CreateStreamableHTTPServer creates a new StreamableHTTP server for the MCP server
func CreateStreamableHTTPServer(mcpServer *server.MCPServer) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(mcpServer)
}
