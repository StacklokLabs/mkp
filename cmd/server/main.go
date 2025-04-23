package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Parse command line flags
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file. If not provided, in-cluster config will be used")
	addr := flag.String("addr", ":8080", "Address to listen on")
	flag.Parse()

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Create Kubernetes client
	k8sClient, err := k8s.NewClient(*kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create MCP implementation
	impl := mcp.NewImplementation(k8sClient)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"kubernetes-mcp-server",
		"0.1.0",
		server.WithResourceCapabilities(true, true),
		server.WithToolCapabilities(true),
	)

	// Add tools
	mcpServer.AddTool(mcp.NewListResourcesTool(), impl.HandleListResources)
	mcpServer.AddTool(mcp.NewApplyResourceTool(), impl.HandleApplyResource)

	// Add resource templates
	mcpServer.AddResourceTemplate(
		mcp.NewClusteredResourceTemplate(),
		impl.HandleClusteredResource,
	)
	mcpServer.AddResourceTemplate(
		mcp.NewNamespacedResourceTemplate(),
		impl.HandleNamespacedResource,
	)

	// Create and start SSE server
	sseServer := mcp.CreateSSEServer(mcpServer)
	log.Printf("Starting MCP server on %s", *addr)
	if err := sseServer.Start(*addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down server...")
	if err := sseServer.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server shutdown complete")
}