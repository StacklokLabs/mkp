package mcp

import (
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/StacklokLabs/mkp/pkg/types"
)

func TestNewListResourcesTool(t *testing.T) {
	tool := NewListResourcesTool()

	// Verify the tool name
	assert.Equal(t, types.ListResourcesToolName, tool.Name, "Tool name should be 'list_resources'")

	// Verify the tool has the required parameters
	schema := tool.InputSchema

	// Check that the schema has the correct type
	assert.Equal(t, "object", schema.Type, "Schema type should be 'object'")

	// Check that the required parameters are in the Required field
	assert.Contains(t, schema.Required, "resource_type", "resource_type should be required")
	assert.Contains(t, schema.Required, "version", "version should be required")
	assert.Contains(t, schema.Required, "resource", "resource should be required")

	// Check that the properties exist
	_, ok := schema.Properties["resource_type"]
	assert.True(t, ok, "Should have 'resource_type' parameter")

	_, ok = schema.Properties["version"]
	assert.True(t, ok, "Should have 'version' parameter")

	_, ok = schema.Properties["resource"]
	assert.True(t, ok, "Should have 'resource' parameter")
}

func TestNewApplyResourceTool(t *testing.T) {
	tool := NewApplyResourceTool()

	// Verify the tool name
	assert.Equal(t, types.ApplyResourceToolName, tool.Name, "Tool name should be 'apply_resource'")

	// Verify the tool has the required parameters
	schema := tool.InputSchema

	// Check that the schema has the correct type
	assert.Equal(t, "object", schema.Type, "Schema type should be 'object'")

	// Check that the required parameters are in the Required field
	assert.Contains(t, schema.Required, "resource_type", "resource_type should be required")
	assert.Contains(t, schema.Required, "version", "version should be required")
	assert.Contains(t, schema.Required, "resource", "resource should be required")

	// Check that the properties exist
	_, ok := schema.Properties["resource_type"]
	assert.True(t, ok, "Should have 'resource_type' parameter")

	_, ok = schema.Properties["version"]
	assert.True(t, ok, "Should have 'version' parameter")

	_, ok = schema.Properties["resource"]
	assert.True(t, ok, "Should have 'resource' parameter")

	// Check manifest parameter
	_, ok = schema.Properties["manifest"]
	assert.True(t, ok, "Should have 'manifest' parameter")
	assert.Contains(t, schema.Required, "manifest", "manifest should be required")
}

func TestNewClusteredResourceTemplate(t *testing.T) {
	template := NewClusteredResourceTemplate()

	// Verify the template URI
	expectedURI := "k8s://clustered/{group}/{version}/{resource}/{name}"
	assert.Equal(t, expectedURI, template.URITemplate.Raw(), "URI template should match")

	// Verify the template name
	expectedName := "Kubernetes Clustered Resource"
	assert.Equal(t, expectedName, template.Name, "Template name should match")

	// Verify the template MIME type
	expectedMIMEType := "application/json"
	assert.Equal(t, expectedMIMEType, template.MIMEType, "MIME type should match")

	// Verify the template description
	expectedDescription := "A Kubernetes clustered resource"
	assert.Equal(t, expectedDescription, template.Description, "Description should match")
}

func TestNewNamespacedResourceTemplate(t *testing.T) {
	template := NewNamespacedResourceTemplate()

	// Verify the template URI
	expectedURI := "k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}"
	assert.Equal(t, expectedURI, template.URITemplate.Raw(), "URI template should match")

	// Verify the template name
	expectedName := "Kubernetes Namespaced Resource"
	assert.Equal(t, expectedName, template.Name, "Template name should match")

	// Verify the template MIME type
	expectedMIMEType := "application/json"
	assert.Equal(t, expectedMIMEType, template.MIMEType, "MIME type should match")

	// Verify the template description
	expectedDescription := "A Kubernetes namespaced resource"
	assert.Equal(t, expectedDescription, template.Description, "Description should match")
}

func TestToolAnnotations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool        mcp.Tool
		readOnly    bool
		destructive bool
		idempotent  bool
	}{
		{tool: NewListResourcesTool(), readOnly: true, destructive: false, idempotent: true},
		{tool: NewGetResourceTool(), readOnly: true, destructive: false, idempotent: true},
		{tool: NewApplyResourceTool(), readOnly: false, destructive: true, idempotent: true},
		{tool: NewDeleteResourceTool(), readOnly: false, destructive: true, idempotent: true},
		{tool: NewPostResourceTool(), readOnly: false, destructive: true, idempotent: false},
	}

	for _, tt := range tests {
		t.Run(tt.tool.Name, func(t *testing.T) {
			t.Parallel()

			// Round-trip through JSON so the test checks what clients see.
			// A nil hint is omitted from the wire and clients fall back to
			// the spec defaults, so every hint must be present.
			raw, err := json.Marshal(tt.tool)
			require.NoError(t, err)
			var wire struct {
				Title       string          `json:"title"`
				Annotations map[string]any  `json:"annotations"`
				InputSchema json.RawMessage `json:"inputSchema"`
			}
			require.NoError(t, json.Unmarshal(raw, &wire))

			assert.NotEmpty(t, wire.Title, "tool should have a top-level title")
			assert.Equal(t, wire.Title, wire.Annotations["title"],
				"top-level title and annotation title should match")
			assert.Equal(t, tt.readOnly, wire.Annotations["readOnlyHint"])
			assert.Equal(t, tt.destructive, wire.Annotations["destructiveHint"])
			assert.Equal(t, tt.idempotent, wire.Annotations["idempotentHint"])
			assert.Equal(t, false, wire.Annotations["openWorldHint"])
		})
	}
}

func TestListResourcesLimitSchema(t *testing.T) {
	t.Parallel()

	limit, ok := NewListResourcesTool().InputSchema.Properties["limit"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integer", limit["type"])
	assert.Equal(t, 0, limit["minimum"])
	assert.Equal(t, 500, limit["maximum"])
}
