package k8s

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// PodLogsFunc is a function type for getting pod logs
type PodLogsFunc func(ctx context.Context, namespace, name string, parameters map[string]string) (*unstructured.Unstructured, error)

// Client represents a Kubernetes client with discovery and dynamic capabilities
type Client struct {
	discoveryClient discovery.DiscoveryInterface
	dynamicClient   dynamic.Interface
	clientset       kubernetes.Interface
	getPodLogs      PodLogsFunc
	kubeconfigPath  string
	mu              sync.RWMutex // Protects access to client components
	
	// For periodic refresh
	refreshCtx       context.Context
	refreshCancel    context.CancelFunc
	refreshInterval  time.Duration
	refreshing       bool
	refreshMu        sync.Mutex // Protects refreshing state
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := getConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %w", err)
	}

	// Create discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create clientset for typed API access
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	client := &Client{
		discoveryClient: discoveryClient,
		dynamicClient:   dynamicClient,
		clientset:       clientset,
		kubeconfigPath:  kubeconfigPath,
	}
	
	// Set the default implementation for getPodLogs
	client.getPodLogs = client.defaultGetPodLogs
	
	return client, nil
}

// ConfigGetter is a function type for getting Kubernetes client configuration
type ConfigGetter func(kubeconfigPath string) (*rest.Config, error)

// defaultConfigGetter is the default implementation of ConfigGetter
func defaultConfigGetter(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		// Try in-cluster config first
		config, err := rest.InClusterConfig()
		if err == nil {
			return config, nil
		}

		// Fall back to default kubeconfig path
		if home := homedir.HomeDir(); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// Use the provided or default kubeconfig path
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// getConfig is the current ConfigGetter implementation
var getConfig ConfigGetter = defaultConfigGetter

// ListAPIResources returns all API resources supported by the Kubernetes API server
func (c *Client) ListAPIResources(ctx context.Context) ([]*metav1.APIResourceList, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	_, resourcesList, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server resources: %w", err)
	}
	return resourcesList, nil
}

// ListClusteredResources returns all clustered resources of the specified group/version/kind
func (c *Client) ListClusteredResources(ctx context.Context, gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
}

// ListNamespacedResources returns all namespaced resources of the specified group/version/kind in the given namespace
func (c *Client) ListNamespacedResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// ApplyClusteredResource creates or updates a clustered resource
func (c *Client) ApplyClusteredResource(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	name := obj.GetName()
	
	// Check if resource exists
	existing, err := c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Resource exists, update it
		// Set the resource version to ensure we're updating the latest version
		obj.SetResourceVersion(existing.GetResourceVersion())
		return c.dynamicClient.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{})
	}
	
	
	// Resource doesn't exist or error occurred, create it
	return c.dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
}

// GetClusteredResource gets a clustered resource
func (c *Client) GetClusteredResource(ctx context.Context, gvr schema.GroupVersionResource, name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
}

// GetNamespacedResource gets a namespaced resource
func (c *Client) GetNamespacedResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ApplyNamespacedResource creates or updates a namespaced resource
func (c *Client) ApplyNamespacedResource(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	name := obj.GetName()
	
	// Check if resource exists
	existing, err := c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Resource exists, update it
		// Set the resource version to ensure we're updating the latest version
		obj.SetResourceVersion(existing.GetResourceVersion())
		return c.dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	}
	
	// Resource doesn't exist or error occurred, create it
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
}

// DeleteClusteredResource deletes a clustered resource
func (c *Client) DeleteClusteredResource(ctx context.Context, gvr schema.GroupVersionResource, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteNamespacedResource deletes a namespaced resource
func (c *Client) DeleteNamespacedResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// SetDynamicClient sets the dynamic client (for testing purposes)
func (c *Client) SetDynamicClient(dynamicClient dynamic.Interface) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.dynamicClient = dynamicClient
}

// SetDiscoveryClient sets the discovery client (for testing purposes)
func (c *Client) SetDiscoveryClient(discoveryClient discovery.DiscoveryInterface) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.discoveryClient = discoveryClient
}

// SetClientset sets the clientset (for testing purposes)
func (c *Client) SetClientset(clientset kubernetes.Interface) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Store the interface directly, we'll use it through the interface methods
	c.clientset = clientset
}

// SetPodLogsFunc sets the function used to get pod logs (for testing purposes)
func (c *Client) SetPodLogsFunc(getPodLogs PodLogsFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.getPodLogs = getPodLogs
}

// GetPodLogs returns the current pod logs function
func (c *Client) GetPodLogs() PodLogsFunc {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.getPodLogs
}

// IsReady returns true if the client is ready to use
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.discoveryClient != nil && c.dynamicClient != nil && c.clientset != nil
}

// RefreshClient re-reads the kubeconfig and updates the client's components
func (c *Client) RefreshClient() error {
	// Get the updated config
	config, err := getConfig(c.kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to get updated Kubernetes config: %w", err)
	}

	// Create new discovery client
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Create new dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create new clientset for typed API access
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Update the client's components with proper locking
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.discoveryClient = discoveryClient
	c.dynamicClient = dynamicClient
	c.clientset = clientset

	return nil
}

// StartPeriodicRefresh starts a goroutine that periodically refreshes the client's configuration
// The interval specifies how often to refresh the configuration
// Returns an error if refresh is already running
func (c *Client) StartPeriodicRefresh(interval time.Duration) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	
	if c.refreshing {
		return fmt.Errorf("periodic refresh is already running")
	}
	
	// Create a cancellable context for the refresh goroutine
	ctx, cancel := context.WithCancel(context.Background())
	c.refreshCtx = ctx
	c.refreshCancel = cancel
	c.refreshInterval = interval
	c.refreshing = true
	
	// Start the refresh goroutine
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Refresh the client
				if err := c.RefreshClient(); err != nil {
					// Log the error but continue refreshing
					fmt.Printf("Error refreshing Kubernetes client: %v\n", err)
				}
			case <-ctx.Done():
				// Context cancelled, stop refreshing
				return
			}
		}
	}()
	
	return nil
}

// StopPeriodicRefresh stops the periodic refresh goroutine
// Returns an error if refresh is not running
func (c *Client) StopPeriodicRefresh() error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	
	if !c.refreshing {
		return fmt.Errorf("periodic refresh is not running")
	}
	
	// Cancel the refresh context to stop the goroutine
	c.refreshCancel()
	c.refreshing = false
	
	return nil
}

// IsRefreshing returns true if the client is periodically refreshing
func (c *Client) IsRefreshing() bool {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	
	return c.refreshing
}

// GetRefreshInterval returns the current refresh interval
// Returns 0 if refresh is not running
func (c *Client) GetRefreshInterval() time.Duration {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	
	if !c.refreshing {
		return 0
	}
	
	return c.refreshInterval
}