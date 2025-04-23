package k8s

import (
	"context"
	"fmt"

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