package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	testingfake "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/k8s"
)

func TestCreateSSEServer(t *testing.T) {
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
					Name:       "pods",
					Kind:       "Pod",
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
	
	// Set the dynamic client
	mockClient.SetDynamicClient(fakeDynamicClient)

	// Create an MCP server with default config
	mcpServer := CreateServer(mockClient, nil)

	assert.NotNil(t, mcpServer, "MCP server should not be nil")

	// Create an SSE server
	sseServer := CreateSSEServer(mcpServer)

	// Verify the server is not nil
	assert.NotNil(t, sseServer, "SSE server should not be nil")
}