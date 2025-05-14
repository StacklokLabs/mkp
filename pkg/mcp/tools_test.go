package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
