package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestWithImpersonation(t *testing.T) {
	// Create a base client with a minimal rest config (localhost to avoid real connections)
	baseConfig := &rest.Config{
		Host: "https://127.0.0.1:6443",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}

	baseClient := &Client{
		restConfig: baseConfig,
	}

	// Set default implementations so we can test they're initialized
	baseClient.getPodLogs = baseClient.defaultGetPodLogs
	baseClient.execInPod = baseClient.defaultExecInPod

	t.Run("creates impersonated client with correct config", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation(
			"juan@stacklok.com",
			[]string{"eng-debugging", "eng-playground"},
		)

		require.NoError(t, err)
		require.NotNil(t, impersonated)

		// Verify impersonation config is set
		assert.Equal(t, "juan@stacklok.com", impersonated.restConfig.Impersonate.UserName)
		assert.Equal(t, []string{"eng-debugging", "eng-playground"}, impersonated.restConfig.Impersonate.Groups)

		// Verify clients were created
		assert.NotNil(t, impersonated.discoveryClient)
		assert.NotNil(t, impersonated.dynamicClient)
		assert.NotNil(t, impersonated.clientset)

		// Verify base config is unchanged
		assert.Empty(t, baseConfig.Impersonate.UserName, "base config should not be modified")
		assert.Empty(t, baseConfig.Impersonate.Groups, "base config should not be modified")
	})

	t.Run("creates independent client", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation("user1@example.com", nil)
		require.NoError(t, err)

		// The impersonated client should have its own config
		assert.NotSame(t, baseConfig, impersonated.restConfig)
	})

	t.Run("handles nil groups", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation("user@example.com", nil)
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", impersonated.restConfig.Impersonate.UserName)
		assert.Nil(t, impersonated.restConfig.Impersonate.Groups)
	})

	t.Run("handles empty groups", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation("user@example.com", []string{})
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", impersonated.restConfig.Impersonate.UserName)
		assert.Empty(t, impersonated.restConfig.Impersonate.Groups)
	})

	t.Run("fails with nil base config", func(t *testing.T) {
		nilConfigClient := &Client{}
		_, err := nilConfigClient.WithImpersonation("user@example.com", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base rest config is nil")
	})

	t.Run("does not inherit periodic refresh", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation("user@example.com", nil)
		require.NoError(t, err)
		assert.False(t, impersonated.IsRefreshing())
		assert.Empty(t, impersonated.kubeconfigPath)
	})

	t.Run("function overrides are default implementations", func(t *testing.T) {
		impersonated, err := baseClient.WithImpersonation("user@example.com", nil)
		require.NoError(t, err)
		assert.NotNil(t, impersonated.getPodLogs)
		assert.NotNil(t, impersonated.execInPod)
	})

	t.Run("shares parent pod-log semaphore", func(t *testing.T) {
		// The pod-log buffer budget is per-process, not per-impersonation —
		// see GHSA-qw5r-ppcg-f8rj. An impersonated client must reference the
		// SAME semaphore channel as its parent so concurrent in-flight reads
		// across all impersonations stay bounded. Channel values compare
		// equal iff they refer to the same underlying channel.
		baseClient.podLogReadSem = make(chan struct{}, maxConcurrentPodLogReads)
		impersonated, err := baseClient.WithImpersonation("user@example.com", nil)
		require.NoError(t, err)
		assert.True(t, baseClient.podLogReadSem == impersonated.podLogReadSem,
			"impersonated client must share parent's semaphore")
	})
}
