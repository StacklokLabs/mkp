package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"
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

	// ImpersonationJWTIssuer is the expected JWT issuer (iss claim).
	// Only used when ImpersonationJWKSURL is set. Prevents token replay
	// from different issuers sharing the same IdP keys.
	ImpersonationJWTIssuer string

	// ImpersonationJWTAudience is the expected JWT audience (aud claim).
	// Only used when ImpersonationJWKSURL is set. Per OIDC Core Section
	// 3.1.3.7, relying parties must validate the audience.
	ImpersonationJWTAudience string
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		ServeResources:     true,  // Default to serving resources for backward compatibility
		ReadWrite:          false, // Default to read-only mode
		EnableRateLimiting: true,  // Default to enabling rate limiting
	}
}

// Server wraps the MCP server and owns all resources that need cleanup.
// Use Stop() to release background goroutines and other resources.
type Server struct {
	mcpServer *server.MCPServer
	config    *Config

	mu          sync.Mutex
	rateLimiter *ratelimit.RateLimiter
	jwksClient  *identity.JWKSClient
}

// CreateServer creates a new MCP server for Kubernetes and returns a Server
// handle that owns all associated resources. Call Stop() on the returned
// Server to clean up.
func CreateServer(k8sClient *k8s.Client, config *Config) *Server {
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

	s := &Server{config: config}

	// Add rate limiting middleware if enabled
	if config.EnableRateLimiting {
		log.Println("Server rate limiting enabled, initializing rate limiter")
		limiter := ratelimit.GetDefaultRateLimiter()
		s.rateLimiter = limiter

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
			timeoutCtx, cancel := context.WithTimeout(
				context.Background(), defaultCtxTimeout,
			)
			defer cancel()

			apiResources, err := impl.HandleListAllResources(timeoutCtx)
			if err != nil {
				log.Printf("Failed to list resources: %v", err)
				return
			}

			for _, resource := range apiResources {
				mcpServer.AddResource(resource, nil)
			}
		}()
	}

	s.mcpServer = mcpServer
	return s
}

// MCPServer returns the underlying mcp-go MCPServer.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// Stop releases all resources owned by this Server, including rate limiters
// and JWKS background refresh goroutines.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
		s.rateLimiter = nil
	}
	if s.jwksClient != nil {
		s.jwksClient.Stop()
		s.jwksClient = nil
	}
}

// identityConfig returns the identity.Config for the given server config.
// If a JWKS URL is configured, it initializes a JWKS client for JWT validation
// and stores it in the Server for cleanup on shutdown.
func (s *Server) identityConfig(
	ctx context.Context, config *Config,
) (*identity.Config, error) {
	cfg := identity.DefaultConfig()
	if config.ImpersonationUserClaim != "" {
		cfg.UserClaim = config.ImpersonationUserClaim
	}
	if config.ImpersonationGroupsClaim != "" {
		cfg.GroupsClaim = config.ImpersonationGroupsClaim
	}
	if config.ImpersonationJWKSURL != "" {
		log.Printf("Initializing JWKS client for JWT validation from %s",
			config.ImpersonationJWKSURL)
		jwksClient, err := identity.NewJWKSClient(ctx, config.ImpersonationJWKSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize JWKS client: %w", err)
		}
		cfg.JWKSClient = jwksClient
		cfg.Issuer = config.ImpersonationJWTIssuer
		cfg.Audience = config.ImpersonationJWTAudience

		s.mu.Lock()
		s.jwksClient = jwksClient
		s.mu.Unlock()
	}
	return cfg, nil
}

// CreateSSEServer creates a new SSE server for the MCP server.
// When impersonation is enabled, it registers an HTTP context function that
// extracts identity from the Authorization header JWT.
func (s *Server) CreateSSEServer(
	ctx context.Context,
) (*server.SSEServer, error) {
	var opts []server.SSEOption
	if s.config != nil && s.config.EnableImpersonation {
		idCfg, err := s.identityConfig(ctx, s.config)
		if err != nil {
			return nil, err
		}
		opts = append(opts, server.WithSSEContextFunc(
			identity.HTTPContextFunc(idCfg),
		))
	}
	return server.NewSSEServer(s.mcpServer, opts...), nil
}

// CreateStreamableHTTPServer creates a new StreamableHTTP server for the
// MCP server. When impersonation is enabled, it registers an HTTP context
// function that extracts identity from the Authorization header JWT.
func (s *Server) CreateStreamableHTTPServer(
	ctx context.Context,
) (*server.StreamableHTTPServer, error) {
	var opts []server.StreamableHTTPOption
	if s.config != nil && s.config.EnableImpersonation {
		idCfg, err := s.identityConfig(ctx, s.config)
		if err != nil {
			return nil, err
		}
		opts = append(opts, server.WithHTTPContextFunc(
			identity.HTTPContextFunc(idCfg),
		))
	}
	return server.NewStreamableHTTPServer(s.mcpServer, opts...), nil
}
