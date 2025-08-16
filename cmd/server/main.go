// Package main provides the entry point for the mkp server application
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/mcp"
)

const (
	// Transport types
	transportSSE            = "sse"
	transportStreamableHTTP = "streamable-http"
)

func main() {
	// Parse command line flags
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file. If not provided, in-cluster config will be used")
	addr := flag.String("addr", getDefaultAddress(), "Address to listen on")
	serveResources := flag.Bool("serve-resources", false,
		"Whether to serve cluster resources as MCP resources. Setting to false reduces context size for LLMs with large clusters")
	readWrite := flag.Bool("read-write", false,
		"Whether to allow write operations on the cluster. When false, the server operates in read-only mode")
	kubeconfigRefreshInterval := flag.Duration("kubeconfig-refresh-interval", 0,
		"Interval to periodically re-read the kubeconfig (e.g., 5m for 5 minutes). If 0, no refresh will be performed")
	enableRateLimiting := flag.Bool("enable-rate-limiting", true,
		"Whether to enable rate limiting for tool calls. When false, no rate limiting will be applied")
	transport := flag.String("transport", getDefaultTransport(),
		"Transport protocol to use: 'sse' or 'streamable-http'. Can also be set via MCP_TRANSPORT environment variable")

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
		ServeResources:     *serveResources,
		ReadWrite:          *readWrite,
		EnableRateLimiting: *enableRateLimiting,
	}

	// Create MCP server using the helper function
	mcpServer := mcp.CreateServer(k8sClient, config)

	// Create and start the appropriate transport server
	var transportServer interface {
		Start(string) error
		Shutdown(context.Context) error
	}

	switch strings.ToLower(*transport) {
	case transportStreamableHTTP:
		log.Println("Using streamable-http transport")
		transportServer = mcp.CreateStreamableHTTPServer(mcpServer)
	case transportSSE:
		log.Println("Using SSE transport")
		transportServer = mcp.CreateSSEServer(mcpServer)
	default:
		log.Fatalf("Invalid transport: %s. Must be 'sse' or 'streamable-http'", *transport)
	}

	// Channel to receive server errors
	serverErrCh := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting MCP server on %s with %s transport", *addr, *transport)
		if err := transportServer.Start(*addr); err != nil {
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

		// Stop the transport server
		err := transportServer.Shutdown(shutdownCtx)
		if err != nil {
			log.Printf("Error during shutdown: %v", err)
		}

		// Stop the MCP server resources (including rate limiter)
		log.Println("Stopping MCP server resources...")
		mcp.StopServer()

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

// getDefaultAddress returns the address to listen on based on MCP_PORT environment variable.
// If the environment variable is not set, returns ":8080".
// If set, validates that the port is valid and returns ":<port>".
func getDefaultAddress() string {
	defaultPort := ":8080"

	portEnv := os.Getenv("MCP_PORT")
	if portEnv == "" {
		return defaultPort
	}

	port, err := strconv.Atoi(portEnv)
	if err != nil {
		log.Printf("Invalid port number in MCP_PORT environment variable: %v, using default port 8080", err)
		return defaultPort
	}

	// Check if port is within valid range
	if port < 1 || port > 65535 {
		log.Printf("Port %d out of valid range (1-65535), using default port", port)
		return defaultPort
	}

	return fmt.Sprintf(":%d", port)
}

// getDefaultTransport returns the transport to use based on MCP_TRANSPORT environment variable.
// If the environment variable is not set, returns "sse".
// Valid values are "sse" and "streamable-http".
func getDefaultTransport() string {
	defaultTransport := transportSSE

	transportEnv := os.Getenv("MCP_TRANSPORT")
	if transportEnv == "" {
		return defaultTransport
	}

	// Normalize the transport value
	transport := strings.ToLower(strings.TrimSpace(transportEnv))

	// Validate the transport value
	if transport != transportSSE && transport != transportStreamableHTTP {
		log.Printf("Invalid MCP_TRANSPORT: %s, using default: %s",
			transportEnv, defaultTransport)
		return defaultTransport
	}

	return transport
}
