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
	
	// Test cases
	testCases := []struct {
		name         string
		namespace    string
		resourceName string
		subresource  string
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "Get clustered resource",
			namespace:    "",
			resourceName: "test-deployment",
			subresource:  "",
			expectError:  false,
		},
		{
			name:         "Get namespaced resource",
			namespace:    "default",
			resourceName: "test-deployment",
			subresource:  "",
			expectError:  false,
		},
		{
			name:         "Empty resource name",
			namespace:    "default",
			resourceName: "",
			subresource:  "",
			expectError:  true,
			errorMsg:     "resource name cannot be empty",
		},
	}
	
	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the method
			ctx := context.Background()
			result, err := client.GetResource(ctx, deploymentGVR, tc.namespace, tc.resourceName, tc.subresource)
			
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
			}
		})
	}
	
	// Note: The fake client doesn't fully support subresources in the same way as the real client,
	// so we're not testing subresource functionality here. In a real environment, the subresource
	// parameter would be passed to the Get method and the appropriate subresource would be returned.
}