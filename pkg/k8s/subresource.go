package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetResource gets a resource or its subresource
// If subresource is empty, the main resource is returned
func (c *Client) GetResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, subresource string) (*unstructured.Unstructured, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	var result *unstructured.Unstructured
	var err error

	// Special handling for pod logs
	if gvr.Resource == "pods" && subresource == "logs" {
		return c.getPodLogs(ctx, namespace, name)
	}

	if namespace == "" {
		// Clustered resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Get(ctx, name, metav1.GetOptions{}, subresource)
		}
	} else {
		// Namespaced resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{}, subresource)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	return result, nil
}

// defaultGetPodLogs retrieves logs from a pod and returns them as an unstructured object
func (c *Client) defaultGetPodLogs(ctx context.Context, namespace, name string) (*unstructured.Unstructured, error) {
	// We need to use the CoreV1 client for logs, as the dynamic client doesn't handle logs properly
	podLogOpts := corev1.PodLogOptions{}
	
	// Get the REST client for pods
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)
	
	// Execute the request
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod logs: %w", err)
	}
	defer func() {
		if closeErr := podLogs.Close(); closeErr != nil {
			// Just log the error, we can't return it at this point
			fmt.Printf("Error closing pod logs stream: %v\n", closeErr)
		}
	}()

	// Read the logs
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to read pod logs: %w", err)
	}

	// Create an unstructured object with the logs
	result := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"logs": buf.String(),
		},
	}

	return result, nil
}