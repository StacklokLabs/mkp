package k8s

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// MaxExecTimeout is the maximum allowed timeout for exec operations to prevent abuse
const MaxExecTimeout = 1 * time.Minute

// defaultExecInPod is the default implementation of ExecInPodFunc
func (c *Client) defaultExecInPod(
	ctx context.Context,
	namespace, name string,
	command []string,
	container string,
	timeout time.Duration,
) (*unstructured.Unstructured, error) {
	if name == "" {
		return nil, fmt.Errorf("pod name cannot be empty")
	}

	if len(command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Default timeout is 15 seconds
	if timeout <= 0 {
		timeout = 15 * time.Second
	}

	// Cap timeout to prevent abuse
	if timeout > MaxExecTimeout {
		timeout = MaxExecTimeout
	}

	// Create a context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create the exec request
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(name).
		Namespace(namespace).
		SubResource("exec")

	// Set query parameters
	option := &corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}

	// Set container if specified
	if container != "" {
		option.Container = container
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	// Create buffers for stdout and stderr
	var stdout, stderr bytes.Buffer

	// Create the SPDY executor
	exec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", req.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	// Execute the command
	err = exec.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// Create an unstructured object with the result
	result := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"command": command,
			},
			"status": map[string]interface{}{
				"stdout": stdout.String(),
				"stderr": stderr.String(),
				"error":  "",
			},
		},
	}

	// Check if the command was killed due to timeout
	if execCtx.Err() == context.DeadlineExceeded {
		result.Object["status"].(map[string]interface{})["error"] = "command timed out"
		return result, nil
	}

	// Check for other errors
	if err != nil {
		result.Object["status"].(map[string]interface{})["error"] = err.Error()
		return result, nil
	}

	return result, nil
}

// handlePodExec handles the special case of executing commands in pods
func (c *Client) handlePodExec(
	ctx context.Context,
	namespace, name string,
	body map[string]interface{},
) (*unstructured.Unstructured, error) {
	// Extract command from body
	commandInterface, ok := body["command"]
	if !ok {
		return nil, fmt.Errorf("command is required for pod exec")
	}

	// Convert command to string slice
	var command []string
	switch cmd := commandInterface.(type) {
	case []interface{}:
		command = make([]string, len(cmd))
		for i, v := range cmd {
			command[i] = fmt.Sprintf("%v", v)
		}
	case string:
		command = []string{cmd}
	default:
		return nil, fmt.Errorf("invalid command format: %T", commandInterface)
	}

	// Extract container name
	container := ""
	if containerInterface, ok := body["container"]; ok {
		container = fmt.Sprintf("%v", containerInterface)
	}

	// Extract timeout
	timeout := 15 * time.Second
	if timeoutInterface, ok := body["timeout"]; ok {
		switch t := timeoutInterface.(type) {
		case int:
			timeout = time.Duration(t) * time.Second
		case int64:
			timeout = time.Duration(t) * time.Second
		case float64:
			timeout = time.Duration(t) * time.Second
		case string:
			if seconds, err := time.ParseDuration(t); err == nil {
				timeout = seconds
			}
		}
	}

	return c.ExecInPod(ctx, namespace, name, command, container, timeout)
}

// PostResource posts to a resource or its subresource
// If subresource is empty, the main resource is posted to
// parameters is a map of string key-value pairs that can be used to customize the request
func (c *Client) PostResource(
	ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace, name, subresource string,
	body map[string]interface{},
	_ map[string]string,
) (*unstructured.Unstructured, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	// Special handling for pod exec
	if gvr.Resource == "pods" && subresource == "exec" {
		return c.handlePodExec(ctx, namespace, name, body)
	}

	// For other subresources, use the dynamic client
	var result *unstructured.Unstructured
	var err error

	// Create the unstructured object from the body
	obj := &unstructured.Unstructured{Object: body}

	if namespace == "" {
		// Clustered resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{})
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Create(ctx, obj, metav1.CreateOptions{}, subresource)
		}
	} else {
		// Namespaced resource
		if subresource == "" {
			// Main resource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
		} else {
			// Subresource
			result, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{}, subresource)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to post to resource: %w", err)
	}

	return result, nil
}
