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

// Hard caps on pod-log retrieval. An attacker-controlled tools/call request
// would otherwise be able to force the server to buffer arbitrary amounts of
// log data in memory. See GHSA-qw5r-ppcg-f8rj.
const (
	maxPodLogTailLines  int64 = 10_000
	maxPodLogLimitBytes int64 = 64 << 20 // 64 MiB
)

// GetResource gets a resource or its subresource
// If subresource is empty, the main resource is returned
// parameters is a map of string key-value pairs that can be used to customize the request
func (c *Client) GetResource(ctx context.Context,
	gvr schema.GroupVersionResource,
	namespace, name, subresource string,
	parameters map[string]string) (*unstructured.Unstructured, error) {
	if name == "" {
		return nil, fmt.Errorf("resource name cannot be empty")
	}

	var result *unstructured.Unstructured
	var err error

	// Special handling for pod logs
	if gvr.Resource == resourcePods && subresource == fieldLogs {
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
func (c *Client) defaultGetPodLogs(
	ctx context.Context,
	namespace, name string,
	parameters map[string]string) (*unstructured.Unstructured, error) {
	// We need to use the CoreV1 client for logs, as the dynamic client doesn't handle logs properly

	// Bound concurrent in-flight reads so that the per-request memory cap
	// (maxPodLogLimitBytes) translates into a predictable aggregate cap.
	// Respect context cancellation while waiting — a slow client should not
	// pin a slot.
	if c.podLogReadSem != nil {
		select {
		case c.podLogReadSem <- struct{}{}:
			defer func() { <-c.podLogReadSem }()
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting to read pod logs: %w", ctx.Err())
		}
	}

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
		podLogOpts = buildPodLogOpts(&podLogOpts, parameters)
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

	logs, err := readBoundedPodLogs(podLogs, effectivePodLogLimit(podLogOpts.LimitBytes))
	if err != nil {
		return nil, err
	}

	// Create an unstructured object with the logs
	result := &unstructured.Unstructured{
		Object: map[string]interface{}{
			fieldAPIVersion: apiVersionV1,
			fieldKind:       kindPod,
			fieldMetadata: map[string]interface{}{
				fieldName:      name,
				fieldNamespace: namespace,
			},
			fieldLogs: string(logs),
		},
	}

	return result, nil
}

// effectivePodLogLimit returns the per-request byte cap used when reading a
// pod log stream. It is the smaller of the caller-requested LimitBytes (after
// clamping in buildPodLogOpts) and the absolute server-side ceiling.
func effectivePodLogLimit(requested *int64) int64 {
	if requested != nil && *requested > 0 && *requested < maxPodLogLimitBytes {
		return *requested
	}
	return maxPodLogLimitBytes
}

// readBoundedPodLogs copies a pod log stream into memory, rejecting any read
// that would exceed the supplied effective limit. The io.LimitedReader is the
// defence-in-depth that makes this safe regardless of whether the apiserver
// honoured PodLogOptions.LimitBytes; reading one extra byte lets us detect an
// overrun without a second syscall.
func readBoundedPodLogs(stream io.Reader, effectiveLimit int64) ([]byte, error) {
	buf := new(bytes.Buffer)
	limited := &io.LimitedReader{R: stream, N: effectiveLimit + 1}
	n, err := io.Copy(buf, limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read pod logs: %w", err)
	}
	if n > effectiveLimit {
		return nil, fmt.Errorf("pod logs exceed maximum size of %d bytes", effectiveLimit)
	}
	return buf.Bytes(), nil
}

func buildPodLogOpts(podLogOpts *corev1.PodLogOptions, parameters map[string]string) corev1.PodLogOptions {
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

	// Limit bytes / tail lines: clamp attacker-controlled values to safe
	// upper bounds. Non-positive values fall back to the maximum rather than
	// being treated as "unlimited" by the apiserver. See GHSA-qw5r-ppcg-f8rj.
	if v, ok := parsePositiveInt64Param(parameters, "limitBytes", maxPodLogLimitBytes); ok {
		podLogOpts.LimitBytes = &v
	}
	if v, ok := parsePositiveInt64Param(parameters, "tailLines", maxPodLogTailLines); ok {
		podLogOpts.TailLines = &v
	}

	return *podLogOpts
}

// parsePositiveInt64Param reads an int64 parameter, clamping any value outside
// (0, maxValue] to maxValue. The (value, true) return means "use this value";
// (0, false) means "leave the field alone" and is deliberately used for both
// the absent-key and the malformed-input cases, because at the current call
// sites the desired behaviour is identical (fall back to whatever default the
// caller pre-populated). If a future caller needs to distinguish "missing"
// from "invalid", split this into separate parse and clamp steps rather than
// adding a third return state.
func parsePositiveInt64Param(parameters map[string]string, key string, maxValue int64) (int64, bool) {
	raw, ok := parameters[key]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	if v <= 0 || v > maxValue {
		v = maxValue
	}
	return v, true
}
