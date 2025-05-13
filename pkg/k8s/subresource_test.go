package k8s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
	getPodLogsMock := func(ctx context.Context, namespace, name string, parameters map[string]string) (*unstructured.Unstructured, error) {
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
	mockGetPodLogs := func(ctx context.Context, namespace, name string, parameters map[string]string) (*unstructured.Unstructured, error) {
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
