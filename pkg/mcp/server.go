package mcp

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/StacklokLabs/mkp/pkg/identity"
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

	// EnableImpersonation determines whether to enable Kubernetes API impersonation
	// based on authenticated user identity from the Authorization header JWT.
	// When true, each K8s API call will impersonate the authenticated user.
	// When false, all requests use the server's own identity (backward compatible).
	EnableImpersonation bool

	// ImpersonationUserClaim is the JWT claim to use for the Impersonate-User header.
	// Defaults to "email" if empty.
	ImpersonationUserClaim string

	// ImpersonationGroupsClaim is the JWT claim to use for the Impersonate-Group headers.
	// Defaults to "groups" if empty.
	ImpersonationGroupsClaim string

	// ImpersonationJWKSURL is an optional URL to a JWKS endpoint for JWT
	// signature validation. When set, JWTs are validated against the keys
	// from this endpoint (signature + expiration). When empty, JWTs are
	// parsed without validation (trusted proxy mode).
	ImpersonationJWKSURL string
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
	// Create MCP implementation (with or without impersonation)
	var impl *Implementation
	if config.EnableImpersonation {
		log.Println("Kubernetes impersonation enabled")
		impl = NewImplementationWithImpersonation(k8sClient)
	} else {
		impl = NewImplementation(k8sClient)
	}

	options := []server.ServerOption{
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
		// Add timeout middleware to prevent context cancellation errors
		WithTimeoutContext(defaultCtxTimeout),
		server.WithRecovery(),
	}

	// Add rate limiting middleware if enabled
	if config.EnableRateLimiting {
		log.Println("Server rate limiting enabled, initializing rate limiter")
		// Create and store the rate limiter for cleanup
		limiter := ratelimit.GetDefaultRateLimiter()

		// Store the limiter for cleanup when the server is stopped
		resources = &serverResources{
			rateLimiter: limiter,
		}

		// Add the middleware to the server options
		middleware := limiter.Middleware()
		options = append(options, server.WithToolHandlerMiddleware(middleware))
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
	// Clean up resources
	if resources != nil {
		// Stop the rate limiter if it exists
		if resources.rateLimiter != nil {
			resources.rateLimiter.Stop()
		}
	}
}

// identityConfig returns the identity.Config for the given server config.
// If a JWKS URL is configured, it initializes a JWKS client for JWT validation.
func identityConfig(config *Config) (*identity.Config, error) {
	cfg := identity.DefaultConfig()
	if config.ImpersonationUserClaim != "" {
		cfg.UserClaim = config.ImpersonationUserClaim
	}
	if config.ImpersonationGroupsClaim != "" {
		cfg.GroupsClaim = config.ImpersonationGroupsClaim
	}
	if config.ImpersonationJWKSURL != "" {
		log.Printf("Initializing JWKS client for JWT validation from %s", config.ImpersonationJWKSURL)
		jwksClient, err := identity.NewJWKSClient(config.ImpersonationJWKSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWKS client: %w", err)
		}
		cfg.JWKSClient = jwksClient
	}
	return cfg, nil
}

// CreateSSEServer creates a new SSE server for the MCP server.
// When impersonation is enabled, it registers an HTTP context function that
// extracts identity from the Authorization header JWT.
func CreateSSEServer(mcpServer *server.MCPServer, config *Config) (*server.SSEServer, error) {
	var opts []server.SSEOption
	if config != nil && config.EnableImpersonation {
		idCfg, err := identityConfig(config)
		if err != nil {
			return nil, err
		}
		opts = append(opts, server.WithSSEContextFunc(identity.HTTPContextFunc(idCfg)))
	}
	return server.NewSSEServer(mcpServer, opts...), nil
}

// CreateStreamableHTTPServer creates a new StreamableHTTP server for the MCP server.
// When impersonation is enabled, it registers an HTTP context function that
// extracts identity from the Authorization header JWT.
func CreateStreamableHTTPServer(mcpServer *server.MCPServer, config *Config) (*server.StreamableHTTPServer, error) {
	var opts []server.StreamableHTTPOption
	if config != nil && config.EnableImpersonation {
		idCfg, err := identityConfig(config)
		if err != nil {
			return nil, err
		}
		opts = append(opts, server.WithHTTPContextFunc(identity.HTTPContextFunc(idCfg)))
	}
	return server.NewStreamableHTTPServer(mcpServer, opts...), nil
}
