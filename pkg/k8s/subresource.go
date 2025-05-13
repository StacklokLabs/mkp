package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetResource gets a resource or its subresource
// If subresource is empty, the main resource is returned
// parameters is a map of string key-value pairs that can be used to customize the request
func (c *Client) GetResource(ctx context.Context, gvr schema.GroupVersionResource, namespace, name, subresource string, parameters map[string]string) (*unstructured.Unstructured, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	var result *unstructured.Unstructured
	var err error

	// Special handling for pod logs
	if gvr.Resource == "pods" && subresource == "logs" {
		return c.getPodLogs(ctx, namespace, name, parameters)
	}

	// Create GetOptions with parameters
	getOptions := metav1.GetOptions{}

	// Apply parameters to GetOptions
	if parameters != nil {
		// ResourceVersion - when specified with a watch call, shows changes that occur after that particular version of a resource
		if resourceVersion, ok := parameters["resourceVersion"]; ok {
			getOptions.ResourceVersion = resourceVersion
		}
	}

	if namespace == "" {
		// Clustered resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Get(ctx, name, getOptions)
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Get(ctx, name, getOptions, subresource)
		}
	} else {
		// Namespaced resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, getOptions)
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, getOptions, subresource)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	return result, nil
}

// defaultGetPodLogs retrieves logs from a pod and returns them as an unstructured object
func (c *Client) defaultGetPodLogs(ctx context.Context, namespace, name string, parameters map[string]string) (*unstructured.Unstructured, error) {
	// We need to use the CoreV1 client for logs, as the dynamic client doesn't handle logs properly

	// Set reasonable defaults for LLM context window
	// Default to last 100 lines and 32KB limit to avoid overwhelming the LLM context
	defaultTailLines := int64(100)
	defaultLimitBytes := int64(32 * 1024) // 32KB

	podLogOpts := corev1.PodLogOptions{
		TailLines:  &defaultTailLines,
		LimitBytes: &defaultLimitBytes,
	}

	// Apply parameters to PodLogOptions
	// Note we don't follow nor tail the logs since we are not using a watcher,
	// this is an MCP tool call after all.
	if parameters != nil {
		// Container name
		if container, ok := parameters["container"]; ok {
			podLogOpts.Container = container
		}

		// Previous container logs
		if previous, ok := parameters["previous"]; ok {
			previousBool, _ := strconv.ParseBool(previous)
			podLogOpts.Previous = previousBool
		}

		// Since seconds (overrides default tail lines)
		if sinceSeconds, ok := parameters["sinceSeconds"]; ok {
			if seconds, err := strconv.ParseInt(sinceSeconds, 10, 64); err == nil {
				podLogOpts.SinceSeconds = &seconds
				// If sinceSeconds is specified, don't use tail lines
				podLogOpts.TailLines = nil
			}
		}

		// Since time
		if sinceTime, ok := parameters["sinceTime"]; ok {
			if t, err := time.Parse(time.RFC3339, sinceTime); err == nil {
				metaTime := metav1.NewTime(t)
				podLogOpts.SinceTime = &metaTime
			}
		}

		// Timestamps
		if timestamps, ok := parameters["timestamps"]; ok {
			timestampsBool, _ := strconv.ParseBool(timestamps)
			podLogOpts.Timestamps = timestampsBool
		}

		// Limit bytes (overrides default limit)
		if limitBytes, ok := parameters["limitBytes"]; ok {
			if bytes, err := strconv.ParseInt(limitBytes, 10, 64); err == nil {
				podLogOpts.LimitBytes = &bytes
			}
		}

		// Tail lines (overrides default tail lines)
		if tailLines, ok := parameters["tailLines"]; ok {
			if lines, err := strconv.ParseInt(tailLines, 10, 64); err == nil {
				podLogOpts.TailLines = &lines
			}
		}
	}

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
