package mcp

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	testingfake "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

func TestHandleClusteredResource(t *testing.T) {
	// Create a mock k8s client with a fake discovery client
	mockClient := &k8s.Client{}

	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &testingfake.Fake{}}

	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
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

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test ClusterRole
	clusterRole := &unstructured.Unstructured{
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

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "clusterroles", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
		return true, clusterRole, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.ReadResourceRequest{}
	// URI format: k8s://clustered/{group}/{version}/{resource}/{name}
	request.Params.URI = "k8s://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role"

	// Test HandleClusteredResource
	result, err := impl.HandleClusteredResource(context.Background(), request)

	// Verify there was no error
	assert.NoError(t, err, "HandleClusteredResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result has the correct length
	assert.Len(t, result, 1, "Result should have 1 item")

	// Verify the result has the correct URI
	assert.Equal(t, request.Params.URI, result[0].(mcp.TextResourceContents).URI, "Result URI should match request URI")

	// Verify the result has the correct MIME type
	assert.Equal(t, "application/json", result[0].(mcp.TextResourceContents).MIMEType, "Result MIME type should be application/json")

	// Verify the result contains the ClusterRole name
	assert.Contains(t, result[0].(mcp.TextResourceContents).Text, "test-cluster-role", "Result should contain the ClusterRole name")
}

func TestHandleNamespacedResource(t *testing.T) {
	// Create a mock k8s client with a fake discovery client
	mockClient := &k8s.Client{}

	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &testingfake.Fake{}}

	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "services",
					Kind:       "Service",
					Namespaced: true,
				},
			},
		},
	}

	// Set the discovery client
	mockClient.SetDiscoveryClient(fakeDiscoveryClient)

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test service
	service := &unstructured.Unstructured{
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

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "services", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
		return true, service, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.ReadResourceRequest{}
	// URI format: k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}
	// For core API group, the group is empty, but we need to include the slash
	request.Params.URI = "k8s://namespaced/default//v1/services/test-service"

	// Test HandleNamespacedResource
	result, err := impl.HandleNamespacedResource(context.Background(), request)

	// Verify there was no error
	assert.NoError(t, err, "HandleNamespacedResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result has the correct length
	assert.Len(t, result, 1, "Result should have 1 item")

	// Verify the result has the correct URI
	assert.Equal(t, request.Params.URI, result[0].(mcp.TextResourceContents).URI, "Result URI should match request URI")

	// Verify the result has the correct MIME type
	assert.Equal(t, "application/json", result[0].(mcp.TextResourceContents).MIMEType, "Result MIME type should be application/json")

	// Verify the result contains the service name
	assert.Contains(t, result[0].(mcp.TextResourceContents).Text, "test-service", "Result should contain the service name")
}

func TestHandleCoreClusteredResource(t *testing.T) {
	// Create a mock k8s client with a fake discovery client
	mockClient := &k8s.Client{}

	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &testingfake.Fake{}}

	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "persistentvolumes",
					Kind:       "PersistentVolume",
					Namespaced: false,
				},
			},
		},
	}

	// Set the discovery client
	mockClient.SetDiscoveryClient(fakeDiscoveryClient)

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test PersistentVolume
	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "PersistentVolume",
			"metadata": map[string]interface{}{
				"name": "test-pv",
			},
			"spec": map[string]interface{}{
				"capacity": map[string]interface{}{
					"storage": "10Gi",
				},
				"accessModes": []interface{}{
					"ReadWriteOnce",
				},
				"persistentVolumeReclaimPolicy": "Retain",
			},
		},
	}

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "persistentvolumes", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
		return true, pv, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.ReadResourceRequest{}
	// URI format: k8s://clustered/{group}/{version}/{resource}/{name}
	// For core API group, the group is empty, but we need to include the slash
	request.Params.URI = "k8s://clustered//v1/persistentvolumes/test-pv"

	// Test HandleClusteredResource
	result, err := impl.HandleClusteredResource(context.Background(), request)

	// Verify there was no error
	assert.NoError(t, err, "HandleClusteredResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result has the correct length
	assert.Len(t, result, 1, "Result should have 1 item")

	// Verify the result has the correct URI
	assert.Equal(t, request.Params.URI, result[0].(mcp.TextResourceContents).URI, "Result URI should match request URI")

	// Verify the result has the correct MIME type
	assert.Equal(t, "application/json", result[0].(mcp.TextResourceContents).MIMEType, "Result MIME type should be application/json")

	// Verify the result contains the PV name
	assert.Contains(t, result[0].(mcp.TextResourceContents).Text, "test-pv", "Result should contain the PV name")
}

func TestHandleNamespacedResourceSingleSlash(t *testing.T) {
	// Create a mock k8s client with a fake discovery client
	mockClient := &k8s.Client{}

	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &testingfake.Fake{}}

	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "deployments",
					Kind:       "Deployment",
					Namespaced: true,
				},
			},
		},
	}

	// Set the discovery client
	mockClient.SetDiscoveryClient(fakeDiscoveryClient)

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test deployment
	deployment := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deployment",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "test",
					},
				},
			},
		},
	}

	// Add a fake get response
	fakeDynamicClient.PrependReactor("get", "deployments", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
		return true, deployment, nil
	})

	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an implementation
	impl := NewImplementation(mockClient)

	// Create a test request
	request := mcp.ReadResourceRequest{}
	// URI format: k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}
	// Using a single slash for the group/version
	request.Params.URI = "k8s://namespaced/default/apps/v1/deployments/test-deployment"

	// Test HandleNamespacedResource
	result, err := impl.HandleNamespacedResource(context.Background(), request)

	// Verify there was no error
	assert.NoError(t, err, "HandleNamespacedResource should not return an error")

	// Verify the result is not nil
	assert.NotNil(t, result, "Result should not be nil")

	// Verify the result has the correct length
	assert.Len(t, result, 1, "Result should have 1 item")

	// Verify the result has the correct URI
	assert.Equal(t, request.Params.URI, result[0].(mcp.TextResourceContents).URI, "Result URI should match request URI")

	// Verify the result has the correct MIME type
	assert.Equal(t, "application/json", result[0].(mcp.TextResourceContents).MIMEType, "Result MIME type should be application/json")

	// Verify the result contains the deployment name
	assert.Contains(t, result[0].(mcp.TextResourceContents).Text, "test-deployment", "Result should contain the deployment name")
}
func TestParseURI(t *testing.T) {
	// Test cases for parseURI
	testCases := []struct {
		name          string
		uri           string
		prefix        string
		expectedParts []string
		expectError   bool
		errorMessage  string
	}{
		{
			name:          "Valid URI with prefix",
			uri:           "k8s://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			prefix:        "k8s://clustered/",
			expectedParts: []string{"rbac.authorization.k8s.io", "v1", "clusterroles", "test-cluster-role"},
			expectError:   false,
		},
		{
			name:          "Valid URI with empty group",
			uri:           "k8s://clustered//v1/persistentvolumes/test-pv",
			prefix:        "k8s://clustered/",
			expectedParts: []string{"v1", "persistentvolumes", "test-pv"},
			expectError:   false,
		},
		{
			name:         "Invalid URI missing prefix",
			uri:          "invalid://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			prefix:       "k8s://clustered/",
			expectError:  true,
			errorMessage: "invalid URI format: missing prefix k8s://clustered/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parts, err := parseURI(tc.uri, tc.prefix)

			if tc.expectError {
				assert.Error(t, err, "parseURI should return an error")
				assert.Equal(t, tc.errorMessage, err.Error(), "Error message should match")
			} else {
				assert.NoError(t, err, "parseURI should not return an error")
				assert.Equal(t, tc.expectedParts, parts, "Parts should match")
			}
		})
	}
}

func TestParseClusteredResourceURI(t *testing.T) {
	// Test cases for parseClusteredResourceURI
	testCases := []struct {
		name               string
		uri                string
		expectedComponents ResourceURIComponents
		expectError        bool
		errorMessage       string
	}{
		{
			name: "Valid URI with group",
			uri:  "k8s://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			expectedComponents: ResourceURIComponents{
				Group:     "rbac.authorization.k8s.io",
				Version:   "v1",
				Resource:  "clusterroles",
				Name:      "test-cluster-role",
				Namespace: "",
			},
			expectError: false,
		},
		{
			name: "Valid URI with empty group",
			uri:  "k8s://clustered//v1/persistentvolumes/test-pv",
			expectedComponents: ResourceURIComponents{
				Group:     "",
				Version:   "v1",
				Resource:  "persistentvolumes",
				Name:      "test-pv",
				Namespace: "",
			},
			expectError: false,
		},
		{
			name:         "Invalid URI missing prefix",
			uri:          "invalid://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			expectError:  true,
			errorMessage: "invalid URI format: missing prefix k8s://clustered/",
		},
		{
			name:         "Invalid URI with too few parts",
			uri:          "k8s://clustered/v1/clusterroles",
			expectError:  true,
			errorMessage: "invalid URI format: expected at least 3 parts after prefix, got 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			components, err := parseClusteredResourceURI(tc.uri)

			if tc.expectError {
				assert.Error(t, err, "parseClusteredResourceURI should return an error")
				assert.Equal(t, tc.errorMessage, err.Error(), "Error message should match")
			} else {
				assert.NoError(t, err, "parseClusteredResourceURI should not return an error")
				assert.Equal(t, tc.expectedComponents.Group, components.Group, "Group should match")
				assert.Equal(t, tc.expectedComponents.Version, components.Version, "Version should match")
				assert.Equal(t, tc.expectedComponents.Resource, components.Resource, "Resource should match")
				assert.Equal(t, tc.expectedComponents.Name, components.Name, "Name should match")
				assert.Equal(t, tc.expectedComponents.Namespace, components.Namespace, "Namespace should match")
			}
		})
	}
}

func TestParseNamespacedResourceURI(t *testing.T) {
	// Test cases for parseNamespacedResourceURI
	testCases := []struct {
		name               string
		uri                string
		expectedComponents ResourceURIComponents
		expectError        bool
		errorMessage       string
	}{
		{
			name: "Valid URI with group",
			uri:  "k8s://namespaced/default/apps/v1/deployments/test-deployment",
			expectedComponents: ResourceURIComponents{
				Group:     "apps",
				Version:   "v1",
				Resource:  "deployments",
				Name:      "test-deployment",
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name: "Valid URI with empty group",
			uri:  "k8s://namespaced/default//v1/services/test-service",
			expectedComponents: ResourceURIComponents{
				Group:     "",
				Version:   "v1",
				Resource:  "services",
				Name:      "test-service",
				Namespace: "default",
			},
			expectError: false,
		},
		{
			name:         "Invalid URI missing prefix",
			uri:          "invalid://namespaced/default/apps/v1/deployments/test-deployment",
			expectError:  true,
			errorMessage: "invalid URI format: missing prefix k8s://namespaced/",
		},
		{
			name:         "Invalid URI with too few parts",
			uri:          "k8s://namespaced/default/v1/services",
			expectError:  true,
			errorMessage: "invalid URI format: expected at least 4 parts after prefix, got 3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			components, err := parseNamespacedResourceURI(tc.uri)

			if tc.expectError {
				assert.Error(t, err, "parseNamespacedResourceURI should return an error")
				assert.Equal(t, tc.errorMessage, err.Error(), "Error message should match")
			} else {
				assert.NoError(t, err, "parseNamespacedResourceURI should not return an error")
				assert.Equal(t, tc.expectedComponents.Group, components.Group, "Group should match")
				assert.Equal(t, tc.expectedComponents.Version, components.Version, "Version should match")
				assert.Equal(t, tc.expectedComponents.Resource, components.Resource, "Resource should match")
				assert.Equal(t, tc.expectedComponents.Name, components.Name, "Name should match")
				assert.Equal(t, tc.expectedComponents.Namespace, components.Namespace, "Namespace should match")
			}
		})
	}
}

func TestHandleClusteredResourceErrors(t *testing.T) {
	// Test cases for HandleClusteredResource errors
	testCases := []struct {
		name         string
		uri          string
		setupMock    func(mockClient *k8s.Client)
		expectError  bool
		errorMessage string
	}{
		{
			name: "Invalid URI format",
			uri:  "invalid://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			setupMock: func(mockClient *k8s.Client) {
				// No setup needed
			},
			expectError:  true,
			errorMessage: "invalid URI format: missing prefix k8s://clustered/",
		},
		{
			name: "Error getting resource",
			uri:  "k8s://clustered/rbac.authorization.k8s.io/v1/clusterroles/test-cluster-role",
			setupMock: func(mockClient *k8s.Client) {
				// Create a fake dynamic client
				scheme := runtime.NewScheme()
				fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

				// Add a fake get response with error
				fakeDynamicClient.PrependReactor("get", "clusterroles", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("resource not found")
				})

				// Set the dynamic client
				mockClient.SetDynamicClient(fakeDynamicClient)
			},
			expectError:  true,
			errorMessage: "failed to get resource: resource not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock k8s client
			mockClient := &k8s.Client{}

			// Setup the mock
			tc.setupMock(mockClient)

			// Create an implementation
			impl := NewImplementation(mockClient)

			// Create a test request
			request := mcp.ReadResourceRequest{}
			request.Params.URI = tc.uri

			// Test HandleClusteredResource
			result, err := impl.HandleClusteredResource(context.Background(), request)

			if tc.expectError {
				assert.Error(t, err, "HandleClusteredResource should return an error")
				assert.Contains(t, err.Error(), tc.errorMessage, "Error message should contain expected text")
				assert.Nil(t, result, "Result should be nil")
			} else {
				assert.NoError(t, err, "HandleClusteredResource should not return an error")
				assert.NotNil(t, result, "Result should not be nil")
			}
		})
	}
}

func TestHandleNamespacedResourceErrors(t *testing.T) {
	// Test cases for HandleNamespacedResource errors
	testCases := []struct {
		name         string
		uri          string
		setupMock    func(mockClient *k8s.Client)
		expectError  bool
		errorMessage string
	}{
		{
			name: "Invalid URI format",
			uri:  "invalid://namespaced/default/apps/v1/deployments/test-deployment",
			setupMock: func(mockClient *k8s.Client) {
				// No setup needed
			},
			expectError:  true,
			errorMessage: "invalid URI format: missing prefix k8s://namespaced/",
		},
		{
			name: "Error getting resource",
			uri:  "k8s://namespaced/default/apps/v1/deployments/test-deployment",
			setupMock: func(mockClient *k8s.Client) {
				// Create a fake dynamic client
				scheme := runtime.NewScheme()
				fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

				// Add a fake get response with error
				fakeDynamicClient.PrependReactor("get", "deployments", func(action testingfake.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("resource not found")
				})

				// Set the dynamic client
				mockClient.SetDynamicClient(fakeDynamicClient)
			},
			expectError:  true,
			errorMessage: "failed to get resource: resource not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock k8s client
			mockClient := &k8s.Client{}

			// Setup the mock
			tc.setupMock(mockClient)

			// Create an implementation
			impl := NewImplementation(mockClient)

			// Create a test request
			request := mcp.ReadResourceRequest{}
			request.Params.URI = tc.uri

			// Test HandleNamespacedResource
			result, err := impl.HandleNamespacedResource(context.Background(), request)

			if tc.expectError {
				assert.Error(t, err, "HandleNamespacedResource should return an error")
				assert.Contains(t, err.Error(), tc.errorMessage, "Error message should contain expected text")
				assert.Nil(t, result, "Result should be nil")
			} else {
				assert.NoError(t, err, "HandleNamespacedResource should not return an error")
				assert.NotNil(t, result, "Result should not be nil")
			}
		})
	}
}
