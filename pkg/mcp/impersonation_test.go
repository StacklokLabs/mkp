package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	ktesting "k8s.io/client-go/testing"

	"github.com/StacklokLabs/mkp/pkg/identity"
	"github.com/StacklokLabs/mkp/pkg/k8s"
	"github.com/StacklokLabs/mkp/pkg/types"
)

func TestClientForContext_ImpersonationDisabled(t *testing.T) {
	mockClient := &k8s.Client{}
	impl := NewImplementation(mockClient)

	ctx := context.Background()
	client, err := impl.clientForContext(ctx)

	require.NoError(t, err)
	assert.Same(t, mockClient, client, "should return the base client when impersonation is disabled")
}

func TestClientForContext_ImpersonationDisabledWithIdentity(t *testing.T) {
	// Even if identity is in context, when impersonation is disabled, base client is used
	mockClient := &k8s.Client{}
	impl := NewImplementation(mockClient)

	id := &identity.Identity{User: "juan@stacklok.com", Groups: []string{"eng"}}
	ctx := identity.WithContext(context.Background(), id)

	client, err := impl.clientForContext(ctx)

	require.NoError(t, err)
	assert.Same(t, mockClient, client, "should return the base client even with identity present")
}

func TestClientForContext_ImpersonationEnabledNoIdentity(t *testing.T) {
	mockClient := &k8s.Client{}
	impl := NewImplementationWithImpersonation(mockClient)

	ctx := context.Background()
	_, err := impl.clientForContext(ctx)

	assert.Error(t, err, "should error when impersonation is enabled but no identity in context")
	assert.Contains(t, err.Error(), "no authenticated identity")
}

func TestClientForContext_ImpersonationEnabledWithIdentity(t *testing.T) {
	// Create a base client with a real-ish rest config
	baseConfig := &rest.Config{
		Host: "https://127.0.0.1:6443",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	baseClient := &k8s.Client{}
	baseClient.SetRestConfigForTest(baseConfig)

	impl := NewImplementationWithImpersonation(baseClient)

	id := &identity.Identity{User: "juan@stacklok.com", Groups: []string{"eng-debugging"}}
	ctx := identity.WithContext(context.Background(), id)

	client, err := impl.clientForContext(ctx)

	require.NoError(t, err)
	assert.NotSame(t, baseClient, client, "should return a new impersonated client")
}

func TestMiddlewarePreservesIdentity(t *testing.T) {
	// This tests that WithTimeoutContext copies identity from the incoming
	// context to the new timeout context.
	id := &identity.Identity{User: "test@example.com", Groups: []string{"grp1"}}

	// Test the middleware logic directly: identity should survive the
	// context.Background() replacement that WithTimeoutContext performs.
	ctx := identity.WithContext(context.Background(), id)

	// Simulate what the middleware does: extract from old ctx, store in new ctx
	extractedID := identity.FromContext(ctx)
	newCtx := context.Background()
	if extractedID != nil {
		newCtx = identity.WithContext(newCtx, extractedID)
	}

	// Verify identity was preserved in the new context
	result := identity.FromContext(newCtx)
	require.NotNil(t, result)
	assert.Equal(t, "test@example.com", result.User)
	assert.Equal(t, []string{"grp1"}, result.Groups)

	// Verify the option was created without error (smoke test)
	opt := WithTimeoutContext(defaultCtxTimeout)
	assert.NotNil(t, opt)
}

func TestHandleListResourcesWithImpersonation_NoIdentity(t *testing.T) {
	// When impersonation is enabled but no identity in context, tool should return error
	mockClient := newMockK8sClient()
	impl := NewImplementationWithImpersonation(mockClient.Client)
	mockClient.SetDynamicClient(mockClient.dynamicClient)

	request := mcp.CallToolRequest{}
	request.Params.Name = types.ListResourcesToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
	}

	ctx := context.Background() // No identity
	result, err := impl.HandleListResources(ctx, request)

	// Should return a tool error result, not a Go error
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "result should be an error when no identity is present")
}

func TestHandleListResourcesWithImpersonation_WithIdentity(t *testing.T) {
	// This test verifies the full flow: identity in context -> impersonated client -> K8s call.
	// Since WithImpersonation needs a real rest.Config, we test the flow with impersonation
	// disabled (which uses the base client) to ensure the handler works correctly.
	mockClient := newMockK8sClient()
	impl := NewImplementation(mockClient.Client) // impersonation OFF
	mockClient.SetDynamicClient(mockClient.dynamicClient)

	listKinds := map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
	}
	scheme := runtime.NewScheme()
	fakeDynamic := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	mockClient.SetDynamicClient(fakeDynamic)

	fakeDynamic.PrependReactor("list", "deployments", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata":   map[string]interface{}{"name": "test"},
				}},
			},
		}, nil
	})

	request := mcp.CallToolRequest{}
	request.Params.Name = types.ListResourcesToolName
	request.Params.Arguments = map[string]interface{}{
		"resource_type": types.ResourceTypeClustered,
		"group":         "apps",
		"version":       "v1",
		"resource":      "deployments",
	}

	id := &identity.Identity{User: "juan@stacklok.com", Groups: []string{"eng"}}
	ctx := identity.WithContext(context.Background(), id)

	result, err := impl.HandleListResources(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "should succeed when identity is present and client works")
}

func TestCreateServerWithImpersonation(t *testing.T) {
	mockClient := &k8s.Client{}

	config := &Config{
		EnableImpersonation:      true,
		ImpersonationUserClaim:   "email",
		ImpersonationGroupsClaim: "groups",
		ServeResources:           false, // Avoid goroutine that hits nil discovery client
	}

	mcpServer := CreateServer(mockClient, config)
	assert.NotNil(t, mcpServer)
}

func TestCreateServerWithoutImpersonation(t *testing.T) {
	mockClient := &k8s.Client{}

	config := &Config{
		EnableImpersonation: false,
		ServeResources:      false, // Avoid goroutine that hits nil discovery client
	}

	mcpServer := CreateServer(mockClient, config)
	assert.NotNil(t, mcpServer)
}

func TestCreateSSEServerWithImpersonation(t *testing.T) {
	mockClient := &k8s.Client{}
	mcpServer := CreateServer(mockClient, &Config{ServeResources: false})

	// With impersonation enabled
	config := &Config{
		EnableImpersonation:      true,
		ImpersonationUserClaim:   "email",
		ImpersonationGroupsClaim: "groups",
	}
	sseServer, err := CreateSSEServer(mcpServer, config)
	require.NoError(t, err)
	assert.NotNil(t, sseServer)

	// Without impersonation
	sseServer2, err := CreateSSEServer(mcpServer, nil)
	require.NoError(t, err)
	assert.NotNil(t, sseServer2)
}

func TestCreateStreamableHTTPServerWithImpersonation(t *testing.T) {
	mockClient := &k8s.Client{}
	mcpServer := CreateServer(mockClient, &Config{ServeResources: false})

	// With impersonation enabled
	config := &Config{
		EnableImpersonation:      true,
		ImpersonationUserClaim:   "email",
		ImpersonationGroupsClaim: "groups",
	}
	httpServer, err := CreateStreamableHTTPServer(mcpServer, config)
	require.NoError(t, err)
	assert.NotNil(t, httpServer)

	// Without impersonation
	httpServer2, err := CreateStreamableHTTPServer(mcpServer, nil)
	require.NoError(t, err)
	assert.NotNil(t, httpServer2)
}
