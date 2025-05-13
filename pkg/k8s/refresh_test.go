package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"
)

// mockGetConfig is a mock function for getConfig
func mockGetConfig(kubeconfigPath string) (*rest.Config, error) {
	return &rest.Config{
		Host: "https://mock-server",
	}, nil
}

// TestRefreshClient tests the RefreshClient method
func TestRefreshClient(t *testing.T) {
	// Save the original getConfig function
	originalGetConfig := getConfig
	defer func() {
		// Restore the original getConfig function after the test
		getConfig = originalGetConfig
	}()

	// Replace getConfig with our mock function
	getConfig = mockGetConfig

	// Create a test client
	client := &Client{
		kubeconfigPath: "test-kubeconfig",
	}

	// Set initial fake clients
	scheme := runtime.NewScheme()
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeClientset := kubefake.NewSimpleClientset()

	client.SetDiscoveryClient(fakeDiscoveryClient)
	client.SetDynamicClient(fakeDynamicClient)
	client.SetClientset(fakeClientset)

	// Store references to the initial clients
	initialDiscoveryClient := client.discoveryClient
	initialDynamicClient := client.dynamicClient
	initialClientset := client.clientset

	// Refresh the client
	err := client.RefreshClient()
	require.NoError(t, err, "RefreshClient should not return an error")

	// Verify that the clients have been replaced
	assert.NotEqual(t, initialDiscoveryClient, client.discoveryClient, "DiscoveryClient should be replaced")
	assert.NotEqual(t, initialDynamicClient, client.dynamicClient, "DynamicClient should be replaced")
	assert.NotEqual(t, initialClientset, client.clientset, "Clientset should be replaced")
}

// TestStartStopPeriodicRefresh tests the StartPeriodicRefresh and StopPeriodicRefresh methods
func TestStartStopPeriodicRefresh(t *testing.T) {
	// Create a test client
	client := &Client{
		kubeconfigPath: "test-kubeconfig",
	}

	// Set initial fake clients
	scheme := runtime.NewScheme()
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeClientset := kubefake.NewSimpleClientset()

	client.SetDiscoveryClient(fakeDiscoveryClient)
	client.SetDynamicClient(fakeDynamicClient)
	client.SetClientset(fakeClientset)

	// Verify that the client is not refreshing initially
	assert.False(t, client.IsRefreshing(), "Client should not be refreshing initially")
	assert.Equal(t, time.Duration(0), client.GetRefreshInterval(), "Refresh interval should be 0 initially")

	// Start periodic refresh with a short interval
	interval := 100 * time.Millisecond
	err := client.StartPeriodicRefresh(interval)
	require.NoError(t, err, "StartPeriodicRefresh should not return an error")

	// Verify that the client is now refreshing
	assert.True(t, client.IsRefreshing(), "Client should be refreshing after StartPeriodicRefresh")
	assert.Equal(t, interval, client.GetRefreshInterval(), "Refresh interval should be set correctly")

	// Trying to start again should return an error
	err = client.StartPeriodicRefresh(interval)
	assert.Error(t, err, "StartPeriodicRefresh should return an error when already refreshing")

	// Stop periodic refresh
	err = client.StopPeriodicRefresh()
	require.NoError(t, err, "StopPeriodicRefresh should not return an error")

	// Verify that the client is no longer refreshing
	assert.False(t, client.IsRefreshing(), "Client should not be refreshing after StopPeriodicRefresh")

	// Trying to stop again should return an error
	err = client.StopPeriodicRefresh()
	assert.Error(t, err, "StopPeriodicRefresh should return an error when not refreshing")
}

// TestPeriodicRefreshActuallyRefreshes tests that the periodic refresh actually refreshes the client
func TestPeriodicRefreshActuallyRefreshes(t *testing.T) {
	// Save the original getConfig function
	originalGetConfig := getConfig
	defer func() {
		// Restore the original getConfig function after the test
		getConfig = originalGetConfig
	}()

	// Create a counter to track how many times getConfig is called
	refreshCount := 0
	getConfig = func(kubeconfigPath string) (*rest.Config, error) {
		refreshCount++
		return &rest.Config{
			Host: "https://mock-server",
		}, nil
	}

	// Create a test client
	client := &Client{
		kubeconfigPath: "test-kubeconfig",
	}

	// Set initial fake clients
	scheme := runtime.NewScheme()
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	fakeClientset := kubefake.NewSimpleClientset()

	client.SetDiscoveryClient(fakeDiscoveryClient)
	client.SetDynamicClient(fakeDynamicClient)
	client.SetClientset(fakeClientset)

	// Start periodic refresh with a short interval
	interval := 100 * time.Millisecond
	err := client.StartPeriodicRefresh(interval)
	require.NoError(t, err, "StartPeriodicRefresh should not return an error")

	// Wait for a few refresh cycles
	time.Sleep(interval * 3)

	// Stop periodic refresh
	err = client.StopPeriodicRefresh()
	require.NoError(t, err, "StopPeriodicRefresh should not return an error")

	// Verify that getConfig was called multiple times
	assert.Greater(t, refreshCount, 1, "getConfig should be called multiple times during periodic refresh")
}

// TestRefreshClientWithRealClients tests the RefreshClient method with real clients
func TestRefreshClientWithRealClients(t *testing.T) {
	// Skip this test if running in CI or without a kubeconfig
	t.Skip("This test requires a real kubeconfig file")

	// Create a test client with a real kubeconfig
	client, err := NewClient("")
	require.NoError(t, err, "NewClient should not return an error")

	// Store references to the initial clients
	initialDiscoveryClient := client.discoveryClient
	initialDynamicClient := client.dynamicClient
	initialClientset := client.clientset

	// Refresh the client
	err = client.RefreshClient()
	require.NoError(t, err, "RefreshClient should not return an error")

	// Verify that the clients have been replaced
	assert.NotEqual(t, initialDiscoveryClient, client.discoveryClient, "DiscoveryClient should be replaced")
	assert.NotEqual(t, initialDynamicClient, client.dynamicClient, "DynamicClient should be replaced")
	assert.NotEqual(t, initialClientset, client.clientset, "Clientset should be replaced")

	// Verify that the client is still functional
	ctx := context.Background()
	_, err = client.ListAPIResources(ctx)
	assert.NoError(t, err, "ListAPIResources should not return an error after refresh")
}
