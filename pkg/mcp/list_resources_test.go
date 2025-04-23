package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

func TestHandleListResourcesClusteredSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	
	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
	}
	
	fakeDynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	
	// Add a fake list response
	fakeDynamicClient.PrependReactor("list", "clusterroles", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
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
				},
			},
		}
		return true, list, nil
	})
	
	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "list_resources"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "clustered",
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
	}
	
	// Test HandleListResources
	ctx := context.Background()
	result, err := impl.HandleListResources(ctx, request)
	
	// Verify there was no error
	assert.NoError(t, err, "HandleListResources should not return an error")
	
	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")
	
	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")
	
	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-cluster-role", "Result should contain the resource name")
}

func TestHandleListResourcesNamespacedSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	
	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "services"}: "ServiceList",
	}
	
	fakeDynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	
	// Add a fake list response
	fakeDynamicClient.PrependReactor("list", "services", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
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
				},
			},
		}
		return true, list, nil
	})
	
	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "list_resources"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "namespaced",
		"group":         "",
		"version":       "v1",
		"resource":      "services",
		"namespace":     "default",
	}
	
	// Test HandleListResources
	ctx := context.Background()
	result, err := impl.HandleListResources(ctx, request)
	
	// Verify there was no error
	assert.NoError(t, err, "HandleListResources should not return an error")
	
	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")
	
	// Verify the result is successful
	assert.False(t, result.IsError, "Result should not be an error")
	
	// Verify the result contains the resource name
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "test-service", "Result should contain the service name")
}

func TestHandleListResourcesMissingParameters(t *testing.T) {
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
			name:      "Missing resource_type",
			arguments: map[string]interface{}{
				"group":    "apps",
				"version":  "v1",
				"resource": "deployments",
			},
			errorMsg: "resource_type is required",
		},
		{
			name:      "Missing version",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"resource":      "deployments",
			},
			errorMsg: "version is required",
		},
		{
			name:      "Missing resource",
			arguments: map[string]interface{}{
				"resource_type": "clustered",
				"group":         "apps",
				"version":       "v1",
			},
			errorMsg: "resource is required",
		},
		{
			name:      "Missing namespace for namespaced resource",
			arguments: map[string]interface{}{
				"resource_type": "namespaced",
				"group":         "apps",
				"version":       "v1",
				"resource":      "deployments",
			},
			errorMsg: "namespace is required for namespaced resources",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			request := mcp.CallToolRequest{}
			request.Params.Name = "list_resources"
			request.Params.Arguments = tc.arguments
			
			// Test HandleListResources
			ctx := context.Background()
			result, err := impl.HandleListResources(ctx, request)
			
			// Verify there was no error
			assert.NoError(t, err, "HandleListResources should not return an error")
			
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

func TestHandleListResourcesInvalidResourceType(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Create a test request with invalid resource_type
	request := mcp.CallToolRequest{}
	request.Params.Name = "list_resources"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "invalid",
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
	}
	
	// Test HandleListResources
	ctx := context.Background()
	result, err := impl.HandleListResources(ctx, request)
	
	// Verify there was no error
	assert.NoError(t, err, "HandleListResources should not return an error")
	
	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")
	
	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")
	
	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Equal(t, "Invalid resource_type: invalid", textContent.Text, "Error message should match")
}

func TestHandleListResourcesListError(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	
	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
	}
	
	fakeDynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	
	// Add a fake list response with error
	fakeDynamicClient.PrependReactor("list", "clusterroles", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("failed to list resources")
	})
	
	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Create a test request
	request := mcp.CallToolRequest{}
	request.Params.Name = "list_resources"
	request.Params.Arguments = map[string]interface{}{
		"resource_type": "clustered",
		"group":         "rbac.authorization.k8s.io",
		"version":       "v1",
		"resource":      "clusterroles",
	}
	
	// Test HandleListResources
	ctx := context.Background()
	result, err := impl.HandleListResources(ctx, request)
	
	// Verify there was no error
	assert.NoError(t, err, "HandleListResources should not return an error")
	
	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")
	
	// Verify the result is an error
	assert.True(t, result.IsError, "Result should be an error")
	
	// Verify the error message
	textContent, ok := mcp.AsTextContent(result.Content[0])
	assert.True(t, ok, "Content should be TextContent")
	assert.Contains(t, textContent.Text, "Failed to list resources", "Error message should contain 'Failed to list resources'")
}

func TestHandleListAllResourcesSuccess(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	
	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Kind:       "Pod",
					Namespaced: true,
				},
				{
					Name:       "services",
					Kind:       "Service",
					Namespaced: true,
				},
				{
					Name:       "pods/log",
					Kind:       "PodLog",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "deployments",
					Kind:       "Deployment",
					Namespaced: true,
				},
				{
					Name:       "statefulsets",
					Kind:       "StatefulSet",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "clusterroles",
					Kind:       "ClusterRole",
					Namespaced: false,
				},
			},
		},
	}
	
	// Set the discovery client
	mockClient.SetDiscoveryClient(fakeDiscoveryClient)
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Test HandleListAllResources
	ctx := context.Background()
	resources, err := impl.HandleListAllResources(ctx)
	
	// Verify there was no error
	assert.NoError(t, err, "HandleListAllResources should not return an error")
	
	// Verify the result is not nil
	assert.NotNil(t, resources, "Resources should not be nil")
	
	// Verify the number of resources (5 resources, excluding subresources)
	assert.Equal(t, 5, len(resources), "Should have 5 resources")
	
	// Verify the resources include both namespaced and clustered resources
	var hasNamespaced, hasClustered bool
	for _, resource := range resources {
		if resource.Name == "Namespaced Pod" {
			hasNamespaced = true
		}
		if resource.Name == "Clustered ClusterRole" {
			hasClustered = true
		}
	}
	assert.True(t, hasNamespaced, "Should include namespaced resources")
	assert.True(t, hasClustered, "Should include clustered resources")
	
	// Verify the URIs are correctly formatted
	for _, resource := range resources {
		if strings.HasPrefix(resource.Name, "Namespaced") {
			assert.Contains(t, resource.URI, "k8s://namespaced/", "Namespaced resource URI should contain 'k8s://namespaced/'")
		} else {
			assert.Contains(t, resource.URI, "k8s://clustered/", "Clustered resource URI should contain 'k8s://clustered/'")
		}
	}
}

func TestHandleListAllResourcesError(t *testing.T) {
	// Create a mock k8s client
	mockClient := &k8s.Client{}
	
	// Create a fake discovery client that returns an error
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	fakeDiscoveryClient.Fake.AddReactor("*", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("failed to list API resources")
	})
	
	// Set the discovery client
	mockClient.SetDiscoveryClient(fakeDiscoveryClient)
	
	// Create an implementation
	impl := NewImplementation(mockClient)
	
	// Test HandleListAllResources
	ctx := context.Background()
	resources, err := impl.HandleListAllResources(ctx)
	
	// Verify there was an error
	assert.Error(t, err, "HandleListAllResources should return an error")
	
	// Verify the error message
	assert.Contains(t, err.Error(), "failed to list API resources", "Error message should contain 'failed to list API resources'")
	
	// Verify the result is nil
	assert.Nil(t, resources, "Resources should be nil")
}