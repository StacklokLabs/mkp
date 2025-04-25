package k8s

import (
	"context"
	"fmt"
	"path/filepath"

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
	}
	
	// Set the default implementation for getPodLogs
	client.getPodLogs = client.defaultGetPodLogs
	
	return client, nil
}

// getConfig returns a Kubernetes client configuration
func getConfig(kubeconfigPath string) (*rest.Config, error) {
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

// ListAPIResources returns all API resources supported by the Kubernetes API server
func (c *Client) ListAPIResources(ctx context.Context) ([]*metav1.APIResourceList, error) {
	_, resourcesList, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server resources: %w", err)
	}
	return resourcesList, nil
}

// ListClusteredResources returns all clustered resources of the specified group/version/kind
func (c *Client) ListClusteredResources(ctx context.Context, gvr schema.GroupVersionResource) (*unstructured.UnstructuredList, error) {
	return c.dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
}

// ListNamespacedResources returns all namespaced resources of the specified group/version/kind in the given namespace
func (c *Client) ListNamespacedResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	return c.dynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// ApplyClusteredResource creates or updates a clustered resource
func (c *Client) ApplyClusteredResource(ctx context.Context, gvr schema.GroupVersionResource, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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
	return c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
}

// GetNamespacedResource gets a namespaced resource
func (c *Client) GetNamespacedResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (interface{}, error) {
	return c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ApplyNamespacedResource creates or updates a namespaced resource
func (c *Client) ApplyNamespacedResource(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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

// SetDynamicClient sets the dynamic client (for testing purposes)
func (c *Client) SetDynamicClient(dynamicClient dynamic.Interface) {
	c.dynamicClient = dynamicClient
}

// SetDiscoveryClient sets the discovery client (for testing purposes)
func (c *Client) SetDiscoveryClient(discoveryClient discovery.DiscoveryInterface) {
	c.discoveryClient = discoveryClient
}

// SetClientset sets the clientset (for testing purposes)
func (c *Client) SetClientset(clientset kubernetes.Interface) {
	// Store the interface directly, we'll use it through the interface methods
	c.clientset = clientset
}

// SetPodLogsFunc sets the function used to get pod logs (for testing purposes)
func (c *Client) SetPodLogsFunc(getPodLogs PodLogsFunc) {
	c.getPodLogs = getPodLogs
}

// GetPodLogs returns the current pod logs function
func (c *Client) GetPodLogs() PodLogsFunc {
	return c.getPodLogs
}

// IsReady returns true if the client is ready to use
func (c *Client) IsReady() bool {
	return c.discoveryClient != nil && c.dynamicClient != nil && c.clientset != nil
}