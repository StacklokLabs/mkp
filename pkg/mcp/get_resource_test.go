package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

func TestHandleGetResourceClusteredSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "test-deployment" {
			return true, &unstructured.Unstructured{
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
			}, nil
		}
		return false, nil, fmt.Errorf("deployment not found")
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_resource"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "clustered",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"name":          "test-deployment",
	}

	// Test HandleGetResource
	ctx := context.Background()
	result, err := impl.HandleGetResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleGetResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-deployment", "Result should contain the resource name")
}

func TestHandleGetResourceNamespacedSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "test-deployment" && getAction.GetNamespace() == "default" {
			return true, &unstructured.Unstructured{
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
			}, nil
		}
		return false, nil, fmt.Errorf("deployment not found")
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_resource"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"namespace":     "default",
		"name":          "test-deployment",
	}

	// Test HandleGetResource
	ctx := context.Background()
	result, err := impl.HandleGetResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleGetResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name and namespace
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-deployment", "Result should contain the resource name")
	assert.Contains(t, textContent.Text, "default", "Result should contain the namespace")
}

func TestHandleGetResourceWithParameters(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create a mock implementation for getPodLogs that verifies parameters
	mockGetPodLogs := func(ctx context.Context, namespace, name string, parameters map[string]string) (*unstructured.Unstructured, error) {
		// Verify parameters were passed correctly
		assert.Equal(t, "test-pod", name)
		assert.Equal(t, "default", namespace)
		assert.NotNil(t, parameters)

		// Check specific parameters
		container, hasContainer := parameters["container"]
		assert.True(t, hasContainer)
		assert.Equal(t, "my-container", container)

		sinceSeconds, hasSinceSeconds := parameters["sinceSeconds"]
		assert.True(t, hasSinceSeconds)
		assert.Equal(t, "3600", sinceSeconds)

		// Return mock logs
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      name,
					"namespace": namespace,
				},
				"logs": "test logs with parameters",
			},
		}, nil
	}

	// Set our mock implementation
	mockClient.SetPodLogsFunc(mockGetPodLogs)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request with parameters
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_resource"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "",
		"version":       "v1",
		"resource":      "pods",
		"namespace":     "default",
		"name":          "test-pod",
		"subresource":   "logs",
		"parameters": map[string]interface{}{
			"container":    "my-container",
			"sinceSeconds": "3600",
		},
	}

	// Test HandleGetResource
	ctx := context.Background()
	result, err := impl.HandleGetResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleGetResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the logs
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test logs with parameters", "Result should contain the logs")
}

func TestHandleGetResourceWithSubresource(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Add a fake get response for subresource
	// Note: The fake client doesn't fully support subresources, so we're simulating it
	fakeDynamicClient.PrependReactor("get", "deployments/status", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "test-deployment" && getAction.GetNamespace() == "default" {
			return true, &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
					"status": map[string]interface{}{
						"replicas":      int64(3),
						"readyReplicas": int64(2),
					},
				},
			}, nil
		}
		return false, nil, fmt.Errorf("deployment status not found")
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_resource"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"namespace":     "default",
		"name":          "test-deployment",
		"subresource":   "status",
	}

	// Test HandleGetResource
	ctx := context.Background()
	result, err := impl.HandleGetResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleGetResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the status information
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")

	// Parse the JSON to verify the content
	var resultObj map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &resultObj)
	assert.NoError(t, err, "Should be able to parse the JSON result")

	// Check for status field
	status, ok := resultObj["status"].(map[string]interface{})
	assert.True(t, ok, "Result should contain status field")
	assert.Equal(t, float64(3), status["replicas"], "Status should contain replicas field")
	assert.Equal(t, float64(2), status["readyReplicas"], "Status should contain readyReplicas field")
}

func TestHandleGetResourceMissingParameters(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Test cases for missing parameters
	testCases := []struct {
		name      string
		arguments map[string]interface{}
		errorMsg  string
	}{
		{
			name: "Missing resource_type",
			arguments: map[string]interface{}{
				"group":    "apps",
				"version":  "v1",
				"resource": "deployments",
				"name":     "test-deployment",
			},
			errorMsg: "resource_type is required",
		},
		{
			name: "Missing version",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"resource":      "deployments",
				"name":          "test-deployment",
			},
			errorMsg: "version is required",
		},
		{
			name: "Missing resource",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"version":       "v1",
				"name":          "test-deployment",
			},
			errorMsg: "resource is required",
		},
		{
			name: "Missing name",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
			},
			errorMsg: "name is required",
		},
		{
			name: "Missing namespace for namespaced resource",
			arguments: map[string]interface{}{
				"resource_type": "namespaced",
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
				"name":          "test-deployment",
			},
			errorMsg: "namespace is required for namespaced resources",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			request := mcp.CallToolRequest{}
			request.Params.Name = "get_resource"
			request.Params.Arguments = tc.arguments

			// Test HandleGetResource
			ctx := context.Background()
			result, err := impl.HandleGetResource(ctx, request)

			// Verify there was no error
			assert.NoError(t, err, "HandleGetResource should not return an error")

			// Verify the result is not nil
			assert.NotNil(t, result, "Result should not be nil")

			// Verify the result is an error
			assert.True(t, result.IsError, "Result should be an error")

			// Verify the error message
			textContent, ok := mcp.AsTextContent(result.Content[0])
			assert.True(t, ok, "Content should be TextContent")
			assert.Equal(t, tc.errorMsg, textContent.Text, "Error message should match")
		})
	}
}

func TestHandleGetResourceInvalidResourceType(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request with invalid resource_type
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_resource"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "invalid",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"name":          "test-deployment",
	}

	// Test HandleGetResource
	ctx := context.Background()
	result, err := impl.HandleGetResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleGetResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "Invalid resource_type", "Error message should contain 'Invalid resource_type'")
}

func TestNewGetResourceTool(t *testing.T) {
	tool := NewGetResourceTool()

	assert.Equal(t, "get_resource", tool.Name)
	assert.Equal(t, "Get a Kubernetes resource or its subresource", tool.Description)

	// Verify the tool exists
	assert.NotNil(t, tool, "Tool should not be nil")
}
