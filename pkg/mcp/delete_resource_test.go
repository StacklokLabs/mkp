package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/types"
)

func TestHandleDeleteResourceClusteredSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Add a fake delete response
	fakeDynamicClient.PrependReactor("delete", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.DeleteResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
		"name":          "test-cluster-role",
	}

	// Test HandleDeleteResource
	ctx := context.Background()
	result, err := impl.HandleDeleteResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleDeleteResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-cluster-role", "Result should contain the resource name")
}

func TestHandleDeleteResourceNamespacedSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Add a fake delete response
	fakeDynamicClient.PrependReactor("delete", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.DeleteResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeNamespaced,
		"group":         "",
		"version":       "v1",
		"resource":      "services",
		"namespace":     "default",
		"name":          "test-service",
	}

	// Test HandleDeleteResource
	ctx := context.Background()
	result, err := impl.HandleDeleteResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleDeleteResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-service", "Result should contain the resource name")
}

func TestHandleDeleteResourceMissingParameters(t *testing.T) {
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
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"resource":      "deployments",
				"name":          "test-deployment",
			},
			errorMsg: "version is required",
		},
		{
			name: "Missing resource",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"version":       "v1",
				"name":          "test-deployment",
			},
			errorMsg: "resource is required",
		},
		{
			name: "Missing name",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeClustered,
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
			},
			errorMsg: "name is required",
		},
		{
			name: "Missing namespace for namespaced resource",
			arguments: map[string]interface{}{
				"resource_type": types.ResourceTypeNamespaced,
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
			request.Params.Name = types.DeleteResourceToolName
			request.Params.Arguments = tc.arguments

			// Test HandleDeleteResource
			ctx := context.Background()
			result, err := impl.HandleDeleteResource(ctx, request)

			// Verify there was no error
			assert.NoError(t, err, "HandleDeleteResource should not return an error")

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

func TestHandleDeleteResourceInvalidResourceType(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request with invalid resource_type
	request := mcp.CallToolRequest{}
	request.Params.Name = types.DeleteResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "invalid",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"name":          "test-deployment",
	}

	// Test HandleDeleteResource
	ctx := context.Background()
	result, err := impl.HandleDeleteResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleDeleteResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Equal(t, "Invalid resource_type: invalid", textContent.Text, "Error message should match")
}

func TestHandleDeleteResourceDeleteError(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := fake.NewSimpleDynamicClient(scheme)

	// Add a fake delete response with error
	fakeDynamicClient.PrependReactor("delete", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("failed to delete resource")
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.DeleteResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
		"name":          "test-cluster-role",
	}

	// Test HandleDeleteResource
	ctx := context.Background()
	result, err := impl.HandleDeleteResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandleDeleteResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "Failed to delete resource", "Error message should contain 'Failed to delete resource'")
}
