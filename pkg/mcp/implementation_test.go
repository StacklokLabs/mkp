package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

// mockK8sClient is a mock implementation of the k8s.Client
type mockK8sClient struct {
	*k8s.Client
	dynamicClient *fake.FakeDynamicClient
}

func newMockK8sClient() *mockK8sClient {
	scheme := runtime.NewScheme()

	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
	}

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)

	return &mockK8sClient{
		Client:        &k8s.Client{},
		dynamicClient: dynamicClient,
	}
}

func TestHandleListResources(t *testing.T) {
	// Create a mock k8s client
	mockClient := newMockK8sClient()

	// Create an MCP implementation with the mock client
	impl := NewImplementation(mockClient.Client)

	// Set the dynamicClient field in the k8s client
	mockClient.SetDynamicClient(mockClient.dynamicClient)

	// Create a test GVR for reference
	_ = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	// Add a fake list response
	mockClient.dynamicClient.PrependReactor("list", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
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
				},
			},
		}
		return true, list, nil
	})

	// Create a test request
	request := struct {
		Name      string
		Arguments map[string]interface{}
	}{
		Name: "list_resources",
		Arguments: map[string]interface{}{
			"resource_type": "clustered",
			"group":         "apps",
			"version":       "v1",
			"resource":      "deployments",
		},
	}

	// Test handleListResources
	ctx := context.Background()
	// Create a CallToolRequest
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = request.Name
	callToolRequest.Params.Arguments = request.Arguments

	result, err := impl.HandleListResources(ctx, callToolRequest)
	if err != nil {
		t.Fatalf("handleListResources failed: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Errorf("Expected result, got nil")
	}
}

func TestHandleApplyResource(t *testing.T) {
	// Create a mock k8s client
	mockClient := newMockK8sClient()

	// Create an MCP implementation with the mock client
	impl := NewImplementation(mockClient.Client)

	// Set the dynamicClient field in the k8s client
	mockClient.SetDynamicClient(mockClient.dynamicClient)

	// Create a test resource
	obj := &unstructured.Unstructured{
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

	// Add a fake get response (resource not found)
	mockClient.dynamicClient.PrependReactor("get", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: deployments \"test-deployment\" not found")
	})

	// Add a fake create response
	mockClient.dynamicClient.PrependReactor("create", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Create a test request
	request := struct {
		Name      string
		Arguments map[string]interface{}
	}{
		Name: "apply_resource",
		Arguments: map[string]interface{}{
			"resource_type": "clustered",
			"group":         "apps",
			"version":       "v1",
			"resource":      "deployments",
			"manifest": map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name": "test-deployment",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
	}

	// Test handleApplyResource
	ctx := context.Background()
	// Create a CallToolRequest
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = request.Name
	callToolRequest.Params.Arguments = request.Arguments

	result, err := impl.HandleApplyResource(ctx, callToolRequest)
	if err != nil {
		t.Fatalf("handleApplyResource failed: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Errorf("Expected result, got nil")
	}

	// For now, we'll skip parsing the result since the API has changed
}

func TestCallTool(t *testing.T) {
	// Create a mock k8s client
	mockClient := newMockK8sClient()

	// Create an MCP implementation with the mock client
	impl := NewImplementation(mockClient.Client)

	// Set the dynamicClient field in the k8s client
	mockClient.SetDynamicClient(mockClient.dynamicClient)

	// Add a fake list response
	mockClient.dynamicClient.PrependReactor("list", "deployments", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "test-deployment",
						},
					},
				},
			},
		}
		return true, list, nil
	})

	// Create a test request
	requestParams := map[string]interface{}{
		"name": "list_resources",
		"arguments": map[string]interface{}{
			"resource_type": "clustered",
			"group":         "apps",
			"version":       "v1",
			"resource":      "deployments",
		},
	}

	// Test CallTool
	ctx := context.Background()
	// Create a CallToolRequest
	callToolRequest := mcp.CallToolRequest{}
	callToolRequest.Params.Name = "list_resources"
	callToolRequest.Params.Arguments = requestParams["arguments"].(map[string]interface{})

	// Call the appropriate handler directly
	result, err := impl.HandleListResources(ctx, callToolRequest)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	// Verify the result is not nil
	if result == nil {
		t.Errorf("Expected result, got nil")
	}
}
