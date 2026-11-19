package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKubeconfig = `apiVersion: v1
kind: Config
clusters: [{name: c, cluster: {server: "https://203.0.113.1:6443"}}]
users: [{name: u, user: {token: x}}]
contexts: [{name: c, context: {cluster: c, user: u}}]
current-context: c
`

// isolateConfigEnv makes the test independent of the developer's or CI
// runner's own kubeconfig and cluster.
func isolateConfigEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KUBECONFIG", "")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
	return home
}

func writeKubeconfig(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(testKubeconfig), 0o600))
}

func TestDefaultConfigGetter(t *testing.T) {
	t.Run("no configuration anywhere", func(t *testing.T) {
		isolateConfigEnv(t)
		_, err := defaultConfigGetter("")
		assert.ErrorIs(t, err, ErrNoConfig)
	})

	t.Run("KUBECONFIG env var", func(t *testing.T) {
		isolateConfigEnv(t)
		path := filepath.Join(t.TempDir(), "config")
		writeKubeconfig(t, path)
		t.Setenv("KUBECONFIG", path)

		config, err := defaultConfigGetter("")
		require.NoError(t, err)
		assert.Equal(t, "https://203.0.113.1:6443", config.Host)
	})

	t.Run("KUBECONFIG naming missing files", func(t *testing.T) {
		isolateConfigEnv(t)
		t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
		_, err := defaultConfigGetter("")
		assert.ErrorIs(t, err, ErrNoConfig)
	})

	t.Run("home kubeconfig", func(t *testing.T) {
		home := isolateConfigEnv(t)
		writeKubeconfig(t, filepath.Join(home, ".kube", "config"))

		config, err := defaultConfigGetter("")
		require.NoError(t, err)
		assert.Equal(t, "https://203.0.113.1:6443", config.Host)
	})

	t.Run("explicit path", func(t *testing.T) {
		isolateConfigEnv(t)
		path := filepath.Join(t.TempDir(), "config")
		writeKubeconfig(t, path)

		config, err := defaultConfigGetter(path)
		require.NoError(t, err)
		assert.Equal(t, "https://203.0.113.1:6443", config.Host)
	})

	t.Run("explicit missing path is an error, not ErrNoConfig", func(t *testing.T) {
		isolateConfigEnv(t)
		_, err := defaultConfigGetter(filepath.Join(t.TempDir(), "missing"))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNoConfig)
	})
}
