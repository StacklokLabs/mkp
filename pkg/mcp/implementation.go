package mcp

import (
	"context"
	"fmt"

	"github.com/StacklokLabs/mkp/pkg/identity"
	"github.com/StacklokLabs/mkp/pkg/k8s"
)

// Implementation implements the MCP protocol
type Implementation struct {
	k8sClient            *k8s.Client
	impersonationEnabled bool
}

// NewImplementation creates a new MCP implementation
func NewImplementation(k8sClient *k8s.Client) *Implementation {
	return &Implementation{
		k8sClient: k8sClient,
	}
}

// NewImplementationWithImpersonation creates a new MCP implementation with
// Kubernetes impersonation enabled. When enabled, each request must carry
// an authenticated identity in its context; requests without identity are
// rejected.
func NewImplementationWithImpersonation(k8sClient *k8s.Client) *Implementation {
	return &Implementation{
		k8sClient:            k8sClient,
		impersonationEnabled: true,
	}
}

// clientForContext returns the appropriate Kubernetes client for the request.
//
// When impersonation is disabled, it returns the shared base client.
// When impersonation is enabled, it extracts the identity from the context
// and returns a per-request impersonated client. If no identity is found
// and impersonation is enabled, it returns an error (strict mode — anonymous
// access is not allowed when impersonation is on).
func (m *Implementation) clientForContext(ctx context.Context) (*k8s.Client, error) {
	if !m.impersonationEnabled {
		return m.k8sClient, nil
	}

	id := identity.FromContext(ctx)
	if id == nil {
		return nil, fmt.Errorf("impersonation is enabled but no authenticated identity was found in the request; " +
			"ensure requests include a valid Authorization header with a JWT from the authentication proxy")
	}

	return m.k8sClient.WithImpersonation(id.User, id.Groups)
}
