package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/StacklokLabs/mkp/pkg/types"
)

// NewListResourcesTool creates a new list_resources tool
func NewListResourcesTool() mcp.Tool {
	return mcp.NewTool(types.ListResourcesToolName,
		mcp.WithDescription("List Kubernetes resources"),
		mcp.WithString("resource_type",
			mcp.Description("Type of resource to list (clustered or namespaced)"),
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
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "List Kubernetes resources",
			ReadOnlyHint: BoolPtr(true),
		}),
	)
}

// NewApplyResourceTool creates a new apply_resource tool
func NewApplyResourceTool() mcp.Tool {
	return mcp.NewTool(types.ApplyResourceToolName,
		mcp.WithDescription("Apply (create or update) a Kubernetes resource"),
		mcp.WithString("resource_type",
			mcp.Description("Type of resource to apply (clustered or namespaced)"),
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
		mcp.WithObject("manifest",
			mcp.Description("Resource manifest"),
			mcp.Required()),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:          "Apply (create or update) a Kubernetes resource",
			ReadOnlyHint:   BoolPtr(false),
			IdempotentHint: BoolPtr(true),
		}),
	)
}

// NewClusteredResourceTemplate creates a new clustered resource template
func NewClusteredResourceTemplate() mcp.ResourceTemplate {
	return mcp.NewResourceTemplate(
		"k8s://clustered/{group}/{version}/{resource}/{name}",
		"Kubernetes Clustered Resource",
		mcp.WithTemplateDescription("A Kubernetes clustered resource"),
		mcp.WithTemplateMIMEType("application/json"),
	)
}

// NewNamespacedResourceTemplate creates a new namespaced resource template
func NewNamespacedResourceTemplate() mcp.ResourceTemplate {
	return mcp.NewResourceTemplate(
		"k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}",
		"Kubernetes Namespaced Resource",
		mcp.WithTemplateDescription("A Kubernetes namespaced resource"),
		mcp.WithTemplateMIMEType("application/json"),
	)
}
