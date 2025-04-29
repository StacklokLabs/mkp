package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/mcp"
)

func main() {
	// Parse command line flags
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file. If not provided, in-cluster config will be used")
	addr := flag.String("addr", ":8080", "Address to listen on")
	serveResources := flag.Bool("serve-resources", true, "Whether to serve cluster resources as MCP resources. Setting to false can reduce context size for LLMs when working with large clusters")
	readWrite := flag.Bool("read-write", false, "Whether to allow write operations on the cluster. When false, the server operates in read-only mode")
	kubeconfigRefreshInterval := flag.Duration("kubeconfig-refresh-interval", 0, "Interval to periodically re-read the kubeconfig (e.g., 5m for 5 minutes). If 0, no refresh will be performed")
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
	
	// Start periodic refresh if interval is set
	if *kubeconfigRefreshInterval > 0 {
		log.Printf("Starting periodic kubeconfig refresh every %v", *kubeconfigRefreshInterval)
		if err := k8sClient.StartPeriodicRefresh(*kubeconfigRefreshInterval); err != nil {
			log.Fatalf("Failed to start periodic kubeconfig refresh: %v", err)
		}
		// Ensure we stop the refresh when shutting down
		defer func() {
			if err := k8sClient.StopPeriodicRefresh(); err != nil {
				log.Printf("Error stopping periodic kubeconfig refresh: %v", err)
			}
		}()
	}

	// Create MCP server config
	config := &mcp.Config{
		ServeResources: *serveResources,
		ReadWrite:      *readWrite,
	}

	// Create MCP server using the helper function
	mcpServer := mcp.CreateServer(k8sClient, config)

	// Create SSE server
	sseServer := mcp.CreateSSEServer(mcpServer)
	
	// Channel to receive server errors
	serverErrCh := make(chan error, 1)
	
	// Start the server in a goroutine
	go func() {
		log.Printf("Starting MCP server on %s", *addr)
		if err := sseServer.Start(*addr); err != nil {
			log.Printf("Server error: %v", err)
			serverErrCh <- err
		}
	}()
	
	// Wait for either a server error or a shutdown signal
	select {
	case err := <-serverErrCh:
		log.Fatalf("Server failed to start: %v", err)
	case <-ctx.Done():
		log.Println("Shutting down server...")
	}
	
	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	// Attempt to shut down the server gracefully
	shutdownCh := make(chan error, 1)
	go func() {
		log.Println("Initiating server shutdown...")
		err := sseServer.Shutdown(shutdownCtx)
		if err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		shutdownCh <- err
		close(shutdownCh)
	}()
	
	// Wait for shutdown to complete or timeout
	select {
	case err, ok := <-shutdownCh:
		if ok {
			if err != nil {
				log.Printf("Server shutdown error: %v", err)
			} else {
				log.Println("Server shutdown completed gracefully")
			}
		}
	case <-shutdownCtx.Done():
		log.Println("Server shutdown timed out, forcing exit...")
		// Force exit after timeout
		os.Exit(1)
	}
	
	log.Println("Server shutdown complete, exiting...")
	// Ensure we exit the program
	os.Exit(0)
}
