package k8s

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestGetResource(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create test resources
	deploymentGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	// Create a clustered deployment
	clusteredDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test-deployment",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	// Create a namespaced deployment
	namespacedDeployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	// Add the resources to the fake client
	_, err := fakeDynamic.Resource(deploymentGVR).Create(context.Background(), clusteredDeployment, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = fakeDynamic.Resource(deploymentGVR).Namespace("default").Create(context.Background(), namespacedDeployment, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Create a test client
	client := &Client{}
	client.SetDynamicClient(fakeDynamic)

	// Create a fake clientset for pod logs test
	fakeClientset := kubefake.NewSimpleClientset()
	client.SetClientset(fakeClientset)

	// Mock the getPodLogs method
	getPodLogsMock := func(_ context.Context, namespace, name string, _ map[string]string) (*unstructured.Unstructured, error) {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"logs": "test log content",
			},
		}, nil
	}

	// Store the original implementation
	originalGetPodLogs := client.getPodLogs

	// Replace with our mock
	client.getPodLogs = getPodLogsMock

	// Restore the original after the test
	defer func() {
		client.getPodLogs = originalGetPodLogs
	}()

	// Test cases
	testCases := []struct {
		name         string
		namespace    string
		resourceName string
		subresource  string
		expectError  bool
		errorMsg     string
		gvr          schema.GroupVersionResource
		checkLogs    bool
		parameters   map[string]string
	}{
		{
			name:         "Get clustered resource",
			namespace:    "",
			resourceName: "test-deployment",
			subresource:  "",
			expectError:  false,
			gvr:          deploymentGVR,
		},
		{
			name:         "Get namespaced resource",
			namespace:    "default",
			resourceName: "test-deployment",
			subresource:  "",
			expectError:  false,
			gvr:          deploymentGVR,
		},
		{
			name:         "Empty resource name",
			namespace:    "default",
			resourceName: "",
			subresource:  "",
			expectError:  true,
			errorMsg:     "resource name cannot be empty",
			gvr:          deploymentGVR,
		},
		{
			name:         "Get pod logs",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "logs",
			expectError:  false,
			gvr:          schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			checkLogs:    true,
		},
		{
			name:         "Get resource with resourceVersion parameter",
			namespace:    "default",
			resourceName: "test-deployment",
			subresource:  "",
			expectError:  false,
			gvr:          deploymentGVR,
			parameters:   map[string]string{"resourceVersion": "12345"},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the method
			ctx := context.Background()
			result, err := client.GetResource(ctx, tc.gvr, tc.namespace, tc.resourceName, tc.subresource, tc.parameters)

			// Assert expectations
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Verify the resource name
				assert.Equal(t, tc.resourceName, result.GetName())

				// Verify the namespace if applicable
				if tc.namespace != "" {
					assert.Equal(t, tc.namespace, result.GetNamespace())
				}

				// Check for logs if this is a pod logs test
				if tc.checkLogs {
					logs, found, err := unstructured.NestedString(result.Object, "logs")
					assert.NoError(t, err)
					assert.True(t, found)
					assert.Equal(t, "test log content", logs)
				}
			}
		})
	}

	// Note: The fake client doesn't fully support subresources in the same way as the real client,
	// so we're not testing subresource functionality here. In a real environment, the subresource
	// parameter would be passed to the Get method and the appropriate subresource would be returned.
}

func TestGetPodLogs(t *testing.T) {
	// Create a test client
	client := &Client{}

	// Create a mock implementation of getPodLogs
	mockGetPodLogs := func(_ context.Context, namespace, name string, _ map[string]string) (*unstructured.Unstructured, error) {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"logs": "test log output",
			},
		}, nil
	}

	// Set the mock implementation
	client.getPodLogs = mockGetPodLogs

	// Call the method through the GetResource method
	ctx := context.Background()
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	result, err := client.GetResource(ctx, podGVR, "default", "test-pod", "logs", nil)

	// Assert expectations
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-pod", result.GetName())
	assert.Equal(t, "default", result.GetNamespace())

	// Verify the logs field
	logs, found, err := unstructured.NestedString(result.Object, "logs")
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "test log output", logs)
}

// Regression tests for GHSA-qw5r-ppcg-f8rj: pod-log retrieval must clamp
// caller-supplied limitBytes/tailLines and must not buffer arbitrary amounts
// of data from the apiserver into memory.

func TestBuildPodLogOpts_ClampsLimitBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  int64
	}{
		{"within bounds", "65536", 65536},
		{"exact max", "67108864", maxPodLogLimitBytes},
		{"over max -> clamped to max", "2147483647", maxPodLogLimitBytes},
		{"int64 max -> clamped to max", "9223372036854775807", maxPodLogLimitBytes},
		{"negative -> clamped to max", "-1", maxPodLogLimitBytes},
		{"zero -> clamped to max", "0", maxPodLogLimitBytes},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defaultBytes := int64(32 * 1024)
			defaultLines := int64(100)
			opts := corev1.PodLogOptions{
				LimitBytes: &defaultBytes,
				TailLines:  &defaultLines,
			}
			got := buildPodLogOpts(&opts, map[string]string{"limitBytes": tc.input})
			require.NotNil(t, got.LimitBytes)
			assert.Equal(t, tc.want, *got.LimitBytes)
		})
	}
}

func TestBuildPodLogOpts_ClampsTailLines(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  int64
	}{
		{"within bounds", "500", 500},
		{"exact max", "10000", maxPodLogTailLines},
		{"over max -> clamped to max", "999999999", maxPodLogTailLines},
		{"int64 max -> clamped to max", "9223372036854775807", maxPodLogTailLines},
		{"negative -> clamped to max", "-1", maxPodLogTailLines},
		{"zero -> clamped to max", "0", maxPodLogTailLines},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			defaultBytes := int64(32 * 1024)
			defaultLines := int64(100)
			opts := corev1.PodLogOptions{
				LimitBytes: &defaultBytes,
				TailLines:  &defaultLines,
			}
			got := buildPodLogOpts(&opts, map[string]string{"tailLines": tc.input})
			require.NotNil(t, got.TailLines)
			assert.Equal(t, tc.want, *got.TailLines)
		})
	}
}

func TestBuildPodLogOpts_NonNumericInputLeavesDefaults(t *testing.T) {
	t.Parallel()

	defaultBytes := int64(32 * 1024)
	defaultLines := int64(100)
	opts := corev1.PodLogOptions{
		LimitBytes: &defaultBytes,
		TailLines:  &defaultLines,
	}
	got := buildPodLogOpts(&opts, map[string]string{
		"limitBytes": "not-a-number",
		"tailLines":  "definitely-not-a-number",
	})
	require.NotNil(t, got.LimitBytes)
	require.NotNil(t, got.TailLines)
	assert.Equal(t, defaultBytes, *got.LimitBytes)
	assert.Equal(t, defaultLines, *got.TailLines)
}

func TestEffectivePodLogLimit(t *testing.T) {
	t.Parallel()

	tenKB := int64(10 * 1024)
	overMax := maxPodLogLimitBytes * 2
	negative := int64(-1)
	zero := int64(0)

	cases := []struct {
		name     string
		input    *int64
		expected int64
	}{
		{"nil requested -> server ceiling", nil, maxPodLogLimitBytes},
		{"under ceiling -> requested", &tenKB, tenKB},
		{"over ceiling -> server ceiling", &overMax, maxPodLogLimitBytes},
		{"negative -> server ceiling", &negative, maxPodLogLimitBytes},
		{"zero -> server ceiling", &zero, maxPodLogLimitBytes},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, effectivePodLogLimit(tc.input))
		})
	}
}

func TestReadBoundedPodLogs_WithinLimit(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("a", 1024)
	got, err := readBoundedPodLogs(bytes.NewReader([]byte(payload)), 4096)
	require.NoError(t, err)
	assert.Equal(t, payload, string(got))
}

func TestReadBoundedPodLogs_RejectsOverrun(t *testing.T) {
	t.Parallel()

	// Apiserver streams more than the effective limit — defence-in-depth
	// must reject rather than buffering the whole stream.
	payload := strings.Repeat("a", 10*1024)
	_, err := readBoundedPodLogs(bytes.NewReader([]byte(payload)), 1024)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceed maximum size")
}

func TestReadBoundedPodLogs_ExactlyAtLimit(t *testing.T) {
	t.Parallel()

	payload := strings.Repeat("a", 1024)
	got, err := readBoundedPodLogs(bytes.NewReader([]byte(payload)), 1024)
	require.NoError(t, err)
	assert.Equal(t, payload, string(got))
}

// trackingReader counts how many bytes were actually read off the wire so we
// can prove the LimitReader stops the read early instead of buffering the
// entire malicious stream.
type trackingReader struct {
	src       *bytes.Reader
	bytesRead int64
}

func (r *trackingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	r.bytesRead += int64(n)
	return n, err
}

func TestReadBoundedPodLogs_DoesNotBufferBeyondLimit(t *testing.T) {
	t.Parallel()

	// 4 MiB of payload, but the effective limit is 1 KiB. The reader must
	// stop reading shortly after 1 KiB rather than draining the whole stream.
	payload := bytes.Repeat([]byte("a"), 4*1024*1024)
	tracker := &trackingReader{src: bytes.NewReader(payload)}

	_, err := readBoundedPodLogs(tracker, 1024)
	require.Error(t, err)
	// We allow one extra byte (the overrun-detection sentinel) but nothing more.
	assert.LessOrEqual(t, tracker.bytesRead, int64(1024+1),
		"LimitedReader must stop the stream at effectiveLimit+1; read %d bytes", tracker.bytesRead)
}

func TestDefaultGetPodLogs_SemaphoreRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	// Pre-fill a 1-slot semaphore so the next acquire would block.
	sem := make(chan struct{}, 1)
	sem <- struct{}{}
	client := &Client{podLogReadSem: sem}

	// Cancel the context before calling so the select picks the ctx.Done()
	// branch deterministically.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.defaultGetPodLogs(ctx, "default", "test-pod", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled",
		"acquire path must surface the context error rather than blocking")
}
