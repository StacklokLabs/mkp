// Package types contains common type definitions and constants used across the MCP implementation
package types

// Constants for resource types
const (
	ResourceTypeNamespaced = "namespaced"
	ResourceTypeClustered  = "clustered"

	// ApplyResourceToolName is the name of the apply_resource tool for tests
	ApplyResourceToolName = "apply_resource"

	// DeleteResourceToolName is the name of the delete_resource tool for tests
	DeleteResourceToolName = "delete_resource"

	// GetResourceToolName is the name of the get_resource tool for tests
	GetResourceToolName = "get_resource"

	// ListResourcesToolName is the name of the list_resources tool for tests
	ListResourcesToolName = "list_resources"

	// PostResourceToolName is the name of the post_resource tool for tests
	PostResourceToolName = "post_resource"
)
