// Package mcp implements the Model, Channel, Prompt (MCP) protocol services
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleDeleteResource handles the delete_resource tool
func (m *Implementation) HandleDeleteResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")
	name := mcp.ParseString(request, "name", "")

	// Validate parameters
	if resourceType == "" {
		return mcp.NewToolResultError("resource_type is required"), nil
	}
	if version == "" {
		return mcp.NewToolResultError("version is required"), nil
	}
	if resource == "" {
		return mcp.NewToolResultError("resource is required"), nil
	}
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if resourceType == ResourceTypeNamespaced && namespace == "" {
		return mcp.NewToolResultError("namespace is required for namespaced resources"), nil
	}

	// Create GVR
	// Validate resource_type
	if resourceType != ResourceTypeClustered && resourceType != ResourceTypeNamespaced {
		return mcp.NewToolResultError("Invalid resource_type: " + resourceType), nil
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Delete resource
	var err error
	switch resourceType {
	case ResourceTypeClustered:
		err = m.k8sClient.DeleteClusteredResource(ctx, gvr, name)
	case ResourceTypeNamespaced:
		err = m.k8sClient.DeleteNamespacedResource(ctx, gvr, namespace, name)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid resource_type: %s", resourceType)), nil
	}

	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to delete resource", err), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted %s resource %s", resourceType, name)), nil
}

// NewDeleteResourceTool creates a new delete_resource tool
func NewDeleteResourceTool() mcp.Tool {
	return mcp.NewTool("delete_resource",
		mcp.WithDescription("Delete a Kubernetes resource"),
		mcp.WithString("resource_type",
			mcp.Description("Type of resource to delete (clustered or namespaced)"),
			mcp.Required()),
		mcp.WithString("group",
			mcp.Description("API group (e.g., apps, networking.k8s.io)")),
		mcp.WithString("version",
			mcp.Description("API version (e.g., v1, v1beta1)"),
			mcp.Required()),
		mcp.WithString("resource",
			mcp.Description("Resource name (e.g., deployments, services)"),
			mcp.Required()),
		mcp.WithString("namespace",
			mcp.Description("Namespace (required for namespaced resources)")),
		mcp.WithString("name",
			mcp.Description("Name of the resource to delete"),
			mcp.Required()),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:          "Delete a Kubernetes resource",
			ReadOnlyHint:   BoolPtr(false),
			IdempotentHint: BoolPtr(true),
		}),
	)
}
