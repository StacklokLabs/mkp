package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/types"
)

func TestHandleApplyResourceClusteredSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
			},
			"rules": []interface{}{
				map[string]interface{}{
					"apiGroups": []interface{}{""},
					"resources": []interface{}{"pods"},
					"verbs":     []interface{}{"get", "list", "watch"},
				},
			},
		},
	}

	// Add a fake get response (resource not found)
	fakeDynamicClient.PrependReactor("get", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: clusterroles \"test-cluster-role\" not found")
	})

	// Add a fake create response
	fakeDynamicClient.PrependReactor("create", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.ApplyResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
		"manifest": map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
			},
			"rules": []interface{}{
				map[string]interface{}{
					"apiGroups": []interface{}{""},
					"resources": []interface{}{"pods"},
					"verbs":     []interface{}{"get", "list", "watch"},
				},
			},
		},
	}

	// Test HandleApplyResource
	ctx := context.Background()
	result, err := impl.HandleApplyResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleApplyResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-cluster-role", "Result should contain the resource name")
}

func TestHandleApplyResourceNamespacedSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
		},
	}

	// Add a fake get response (resource not found)
	fakeDynamicClient.PrependReactor("get", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: services \"test-service\" not found")
	})

	// Add a fake create response
	fakeDynamicClient.PrependReactor("create", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.ApplyResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeNamespaced,
		"group":         "",
		"version":       "v1",
		"resource":      "services",
		"namespace":     "default",
		"manifest": map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
		},
	}

	// Test HandleApplyResource
	ctx := context.Background()
	result, err := impl.HandleApplyResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleApplyResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-service", "Result should contain the resource name")
}

func TestHandleApplyResourceMissingParameters(t *testing.T) {
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
				"manifest": map[string]interface{}{},
			},
			errorMsg: "resource_type is required",
		},
		{
			name: "Missing version",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"resource":      "deployments",
				"manifest":      map[string]interface{}{},
			},
			errorMsg: "version is required",
		},
		{
			name: "Missing resource",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"version":       "v1",
				"manifest":      map[string]interface{}{},
			},
			errorMsg: "resource is required",
		},
		{
			name: "Missing namespace for namespaced resource",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeNamespaced,
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
				"manifest":      map[string]interface{}{},
			},
			errorMsg: "namespace is required for namespaced resources",
		},
		{
			name: "Missing manifest",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
			},
			errorMsg: "manifest is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			request := mcp.CallToolRequest{}
			request.Params.Name = types.ApplyResourceToolName
			request.Params.Arguments = tc.arguments

			// Test HandleApplyResource
			ctx := context.Background()
			result, err := impl.HandleApplyResource(ctx, request)

			// Verify there was no error
			assert.NoError(t, err, "HandleApplyResource should not return an error")

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

func TestHandleApplyResourceInvalidResourceType(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request with invalid resource_type
	request := mcp.CallToolRequest{}
	request.Params.Name = types.ApplyResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "invalid",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"manifest":      map[string]interface{}{},
	}

	// Test HandleApplyResource
	ctx := context.Background()
	result, err := impl.HandleApplyResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleApplyResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Equal(t, "Invalid resource_type: invalid", textContent.Text, "Error message should match")
}

func TestHandleApplyResourceApplyError(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Add a fake get response (resource not found)
	fakeDynamicClient.PrependReactor("get", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: clusterroles \"test-cluster-role\" not found")
	})

	// Add a fake create response with error
	fakeDynamicClient.PrependReactor("create", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("failed to create resource")
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.ApplyResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
		"manifest": map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
			},
		},
	}

	// Test HandleApplyResource
	ctx := context.Background()
	result, err := impl.HandleApplyResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleApplyResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "Failed to apply resource", "Error message should contain 'Failed to apply resource'")
}
