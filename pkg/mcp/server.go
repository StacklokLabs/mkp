package mcp

import (
	"context"
	"log"

	"github.com/mark3labs/mcp-go/server"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

// CreateServer creates a new MCP server for Kubernetes
func CreateServer(k8sClient *k8s.Client) *server.MCPServer {
	// Create MCP implementation
	impl := NewImplementation(k8sClient)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"kubernetes-mcp-server",
		"0.1.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	// Add tools
	mcpServer.AddTool(NewListResourcesTool(), impl.HandleListResources)
	mcpServer.AddTool(NewApplyResourceTool(), impl.HandleApplyResource)
	mcpServer.AddTool(NewGetResourceTool(), impl.HandleGetResource)

	// Add resource templates
	mcpServer.AddResourceTemplate(
		NewClusteredResourceTemplate(),
		impl.HandleClusteredResource,
	)
	mcpServer.AddResourceTemplate(
		NewNamespacedResourceTemplate(),
		impl.HandleNamespacedResource,
	)

	// Add resources
	go func() {
		// List resources in a goroutine to avoid blocking server startup
		resources, err := impl.HandleListAllResources(context.Background())
		if err != nil {
			log.Printf("Failed to list resources: %v", err)
			return
		}

		// Add resources to the server
		for _, resource := range resources {
			mcpServer.AddResource(resource, nil)
		}
	}()

	return mcpServer
}

// CreateSSEServer creates a new SSE server for the MCP server
func CreateSSEServer(mcpServer *server.MCPServer) *server.SSEServer {
	return server.NewSSEServer(mcpServer)
}