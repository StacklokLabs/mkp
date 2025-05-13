package mcp

import (
	"github.com/StacklokLabs/mkp/pkg/k8s"
)

// Implementation implements the MCP protocol
type Implementation struct {
	k8sClient *k8s.Client
}

// NewImplementation creates a new MCP implementation
func NewImplementation(k8sClient *k8s.Client) *Implementation {
	return &Implementation{
		k8sClient: k8sClient,
	}
}
