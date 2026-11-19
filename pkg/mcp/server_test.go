package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	// Create an MCP server with ServeResources disabled to avoid
	// background goroutine that can race with test completion
	srv := CreateServer(mockClient, &Config{
		ServeResources:     false,
		EnableRateLimiting: true,
	})
	defer srv.Stop()

	assert.NotNil(t, srv.MCPServer(), "MCP server should not be nil")

	// Create an SSE server (no impersonation)
	sseServer, err := srv.CreateSSEServer(t.Context())
	assert.NoError(t, err, "CreateSSEServer should not return an error")

	// Verify the server is not nil
	assert.NotNil(t, sseServer, "SSE server should not be nil")
}

// TestCreateStreamableHTTPServer checks that the Streamable HTTP server
// accepts requests using MCP 2026-07-28 and runs stateless, so legacy clients
// work without an Mcp-Session-Id.
func TestCreateStreamableHTTPServer(t *testing.T) {
	t.Parallel()

	srv := CreateServer(&k8s.Client{}, &Config{ReadWrite: true})
	t.Cleanup(srv.Stop)

	httpServer, err := srv.CreateStreamableHTTPServer(t.Context())
	require.NoError(t, err)
	ts := httptest.NewServer(httpServer)
	t.Cleanup(ts.Close)

	post := func(t *testing.T, body string, headers map[string]string) (*http.Response, map[string]any) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL+"/mcp", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		t.Cleanup(func() { _ = resp.Body.Close() })
		var msg map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&msg))
		return resp, msg
	}

	t.Run("modern tools/list", func(t *testing.T) {
		t.Parallel()
		_, msg := post(t, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{`+
			`"io.modelcontextprotocol/protocolVersion":"2026-07-28",`+
			`"io.modelcontextprotocol/clientCapabilities":{}}}}`,
			map[string]string{"Mcp-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/list"})

		require.Nil(t, msg["error"], "2026-07-28 request should not be rejected: %v", msg["error"])
		result, ok := msg["result"].(map[string]any)
		require.True(t, ok)
		tools, ok := result["tools"].([]any)
		require.True(t, ok)
		assert.Len(t, tools, 5)
	})

	t.Run("legacy initialize has no session", func(t *testing.T) {
		t.Parallel()
		resp, msg := post(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{`+
			`"protocolVersion":"2025-06-18","capabilities":{},`+
			`"clientInfo":{"name":"test","version":"0.0.1"}}}`, nil)

		require.Nil(t, msg["error"])
		assert.Empty(t, resp.Header.Get("Mcp-Session-Id"))
	})
}
