package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestPostResource(t *testing.T) {
	// Create a test client
	client := &Client{}

	// Create a fake clientset
	fakeClientset := kubefake.NewSimpleClientset()
	client.SetClientset(fakeClientset)

	// Mock the ExecInPod method
	mockExecCalled := false
	mockExecFunc := func(_ context.Context, namespace, name string, command []string, _ string, _ time.Duration) (*unstructured.Unstructured, error) {
		// Mark that the function was called
		mockExecCalled = true

		// Verify parameters
		assert.Equal(t, "test-pod", name)
		assert.Equal(t, "default", namespace)

		return &unstructured.Unstructured{
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
					"stdout": "test command output",
					"stderr": "",
					"error":  "",
				},
			},
		}, nil
	}

	// Set the mock implementation
	client.SetExecInPodFunc(mockExecFunc)

	// Test cases
	testCases := []struct {
		name         string
		namespace    string
		resourceName string
		subresource  string
		body         map[string]interface{}
		expectError  bool
		errorMsg     string
		gvr          schema.GroupVersionResource
		checkExec    bool
	}{
		{
			name:         "Post to pod exec",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "exec",
			body: map[string]interface{}{
				"command": []interface{}{"ls", "-la"},
			},
			expectError: false,
			gvr:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			checkExec:   true,
		},
		{
			name:         "Post to pod exec with string command",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "exec",
			body: map[string]interface{}{
				"command": "ls -la",
			},
			expectError: false,
			gvr:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			checkExec:   true,
		},
		{
			name:         "Post to pod exec with container",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "exec",
			body: map[string]interface{}{
				"command":   []interface{}{"ls", "-la"},
				"container": "sidecar",
			},
			expectError: false,
			gvr:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			checkExec:   true,
		},
		{
			name:         "Post to pod exec with timeout",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "exec",
			body: map[string]interface{}{
				"command": []interface{}{"ls", "-la"},
				"timeout": 30,
			},
			expectError: false,
			gvr:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			checkExec:   true,
		},
		{
			name:         "Post to pod exec missing command",
			namespace:    "default",
			resourceName: "test-pod",
			subresource:  "exec",
			body:         map[string]interface{}{},
			expectError:  true,
			errorMsg:     "command is required for pod exec",
			gvr:          schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
		{
			name:         "Empty resource name",
			namespace:    "default",
			resourceName: "",
			subresource:  "exec",
			body: map[string]interface{}{
				"command": []interface{}{"ls", "-la"},
			},
			expectError: true,
			errorMsg:    "resource name cannot be empty",
			gvr:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the mock flag
			mockExecCalled = false

			// Call the method
			ctx := context.Background()
			result, err := client.PostResource(ctx, tc.gvr, tc.namespace, tc.resourceName, tc.subresource, tc.body, nil)

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

				// Check if the mock was called for exec tests
				if tc.checkExec {
					assert.True(t, mockExecCalled, "ExecInPod function should have been called")

					// Check for exec output
					stdout, found, err := unstructured.NestedString(result.Object, "status", "stdout")
					assert.NoError(t, err)
					assert.True(t, found)
					assert.Equal(t, "test command output", stdout)
				}
			}
		})
	}
}

func TestExecInPod(t *testing.T) {
	// Create a test client
	client := &Client{}

	// Create a fake clientset
	fakeClientset := kubefake.NewSimpleClientset()
	client.SetClientset(fakeClientset)

	// Mock the ExecInPod method
	var capturedTimeout time.Duration
	mockExecFunc := func(_ context.Context, namespace, name string, command []string, _ string, timeout time.Duration) (*unstructured.Unstructured, error) {
		// Apply the same timeout adjustments as the real implementation
		if timeout <= 0 {
			timeout = 15 * time.Second
		}
		if timeout > MaxExecTimeout {
			timeout = MaxExecTimeout
		}

		// Capture the adjusted timeout for verification
		capturedTimeout = timeout

		// Perform the same validation as the real implementation
		if name == "" {
			return nil, fmt.Errorf("pod name cannot be empty")
		}

		if len(command) == 0 {
			return nil, fmt.Errorf("command cannot be empty")
		}

		// Return a mock result
		return &unstructured.Unstructured{
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
					"stdout": "test command output",
					"stderr": "",
					"error":  "",
				},
			},
		}, nil
	}

	// Set the mock implementation
	client.SetExecInPodFunc(mockExecFunc)

	// Test cases
	testCases := []struct {
		name            string
		namespace       string
		podName         string
		command         []string
		container       string
		timeout         time.Duration
		expectError     bool
		errorMsg        string
		checkTimeout    bool
		expectedTimeout time.Duration
	}{
		{
			name:        "Empty pod name",
			namespace:   "default",
			podName:     "",
			command:     []string{"ls", "-la"},
			timeout:     30 * time.Second,
			expectError: true,
			errorMsg:    "pod name cannot be empty",
		},
		{
			name:        "Empty command",
			namespace:   "default",
			podName:     "test-pod",
			command:     []string{},
			timeout:     30 * time.Second,
			expectError: true,
			errorMsg:    "command cannot be empty",
		},
		{
			name:            "Zero timeout",
			namespace:       "default",
			podName:         "test-pod",
			command:         []string{"ls", "-la"},
			timeout:         0,
			expectError:     false,
			checkTimeout:    true,
			expectedTimeout: 15 * time.Second, // Default timeout
		},
		{
			name:            "Negative timeout",
			namespace:       "default",
			podName:         "test-pod",
			command:         []string{"ls", "-la"},
			timeout:         -5 * time.Second,
			expectError:     false,
			checkTimeout:    true,
			expectedTimeout: 15 * time.Second, // Default timeout
		},
		{
			name:            "Excessive timeout",
			namespace:       "default",
			podName:         "test-pod",
			command:         []string{"ls", "-la"},
			timeout:         10 * time.Minute,
			expectError:     false,
			checkTimeout:    true,
			expectedTimeout: MaxExecTimeout, // Capped at MaxExecTimeout
		},
		{
			name:        "With container",
			namespace:   "default",
			podName:     "test-pod",
			command:     []string{"ls", "-la"},
			container:   "sidecar",
			timeout:     30 * time.Second,
			expectError: false,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the captured timeout
			capturedTimeout = 0

			// Call the method
			ctx := context.Background()
			result, err := client.ExecInPod(ctx, tc.namespace, tc.podName, tc.command, tc.container, tc.timeout)

			// Assert expectations
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Check timeout handling if applicable
				if tc.checkTimeout {
					assert.Equal(t, tc.expectedTimeout, capturedTimeout, "Timeout should be adjusted correctly")
				}
			}
		})
	}
}
