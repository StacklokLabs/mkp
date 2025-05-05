package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/types"
)

func TestHandlePostResourceExecSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a mock implementation for ExecInPod
	mockExecCalled := false
	mockExecFunc := func(_ context.Context, namespace, name string, command []string, container string, _ time.Duration) (*unstructured.Unstructured, error) {
		// Mark that the function was called
		mockExecCalled = true

		// Verify parameters were passed correctly
		assert.Equal(t, "test-pod", name)
		assert.Equal(t, "default", namespace)
		assert.Equal(t, []string{"ls", "-la", "/"}, command)
		assert.Equal(t, "my-container", container)

		// Return mock exec result
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
					"stdout": "total 48\ndrwxr-xr-x   1 root root 4096 May  5 14:30 .\ndrwxr-xr-x   1 root root 4096 May  5 14:30 ..\n...",
					"stderr": "",
					"error":  "",
				},
			},
		}, nil
	}

	// Set the mock implementation
	mockClient.SetExecInPodFunc(mockExecFunc)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.PostResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "",
		"version":       "v1",
		"resource":      "pods",
		"namespace":     "default",
		"name":          "test-pod",
		"subresource":   "exec",
		"body": map[string]interface{}{
			"command":   []interface{}{"ls", "-la", "/"},
			"container": "my-container",
			"timeout":   30,
		},
	}

	// Test HandlePostResource
	ctx := context.Background()
	result, err := impl.HandlePostResource(ctx, request)

	// Verify the mock function was called
	assert.True(t, mockExecCalled, "ExecInPod function should have been called")

	// Verify there was no error
	assert.NoError(t, err, "HandlePostResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the expected output
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")

	// Parse the JSON to verify the content
	var resultObj map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &resultObj)
	assert.NoError(t, err, "Should be able to parse the JSON result")

	// Check for status field
	status, ok := resultObj["status"].(map[string]interface{})
	assert.True(t, ok, "Result should contain status field")
	assert.Contains(t, status["stdout"], "total 48", "Status should contain stdout with command output")
	assert.Equal(t, "", status["stderr"], "Status should contain empty stderr")
	assert.Equal(t, "", status["error"], "Status should contain empty error")
}

func TestHandlePostResourceGenericSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Add a fake create response
	fakeDynamicClient.PrependReactor("create", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(ktesting.CreateAction)
		obj := createAction.GetObject().(*unstructured.Unstructured)

		// Verify the object has the expected data
		data, found, err := unstructured.NestedMap(obj.Object, "data")
		assert.True(t, found, "Object should have data field")
		assert.NoError(t, err, "Should be able to get data field")
		assert.Equal(t, "value1", data["key1"], "Data should contain key1=value1")

		// Return the object with a resource version
		obj.SetResourceVersion("1")
		return true, obj, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = types.PostResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "",
		"version":       "v1",
		"resource":      "configmaps",
		"namespace":     "default",
		"name":          "test-configmap",
		"body": map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "test-configmap",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key1": "value1",
			},
		},
	}

	// Test HandlePostResource
	ctx := context.Background()
	result, err := impl.HandlePostResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandlePostResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")

	// Verify the result contains the expected output
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")

	// Parse the JSON to verify the content
	var resultObj map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &resultObj)
	assert.NoError(t, err, "Should be able to parse the JSON result")

	// Check for data field
	data, ok := resultObj["data"].(map[string]interface{})
	assert.True(t, ok, "Result should contain data field")
	assert.Equal(t, "value1", data["key1"], "Data should contain key1=value1")
}

func TestHandlePostResourceMissingParameters(t *testing.T) {
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
				"body":     map[string]interface{}{},
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
				"body":          map[string]interface{}{},
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
				"body":          map[string]interface{}{},
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
				"body":          map[string]interface{}{},
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
				"body":          map[string]interface{}{},
			},
			errorMsg: "namespace is required for namespaced resources",
		},
		{
			name: "Missing body",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
				"name":          "test-deployment",
			},
			errorMsg: "body is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			request := mcp.CallToolRequest{}
			request.Params.Name = types.PostResourceToolName
			request.Params.Arguments = tc.arguments

			// Test HandlePostResource
			ctx := context.Background()
			result, err := impl.HandlePostResource(ctx, request)

			// Verify there was no error
			assert.NoError(t, err, "HandlePostResource should not return an error")

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

func TestHandlePostResourceInvalidResourceType(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request with invalid resource_type
	request := mcp.CallToolRequest{}
	request.Params.Name = types.PostResourceToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "invalid",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
		"name":          "test-deployment",
		"body":          map[string]interface{}{},
	}

	// Test HandlePostResource
	ctx := context.Background()
	result, err := impl.HandlePostResource(ctx, request)

	// Verify there was no error
	assert.NoError(t, err, "HandlePostResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")

	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "invalid resource_type", "Error message should contain 'invalid resource_type'")
}

func TestNewPostResourceTool(t *testing.T) {
	tool := NewPostResourceTool()

	assert.Equal(t, types.PostResourceToolName, tool.Name)
	assert.Equal(t, "Post to a Kubernetes resource or its subresource", tool.Description)

	// Verify the tool exists
	assert.NotNil(t, tool, "Tool should not be nil")
}
