package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/StacklokLabs/mkp/pkg/types"
)

// NewListResourcesTool creates a new list_resources tool
func NewListResourcesTool() mcp.Tool {
	return mcp.NewTool(types.ListResourcesToolName,
		mcp.WithToolTitle(listResourcesTitle),
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
		mcp.WithString("label_selector",
			mcp.Description("Kubernetes label selector for filtering resources (optional)")),
		mcp.WithBoolean("include_annotations",
			mcp.Description("Whether to include annotations in the output (default: true)")),
		mcp.WithArray("exclude_annotation_keys",
			mcp.Description("List of annotation keys to exclude from output (supports wildcards with *)"),
			mcp.Items(map[string]any{
				"type": "string",
			}),
		),
		mcp.WithArray("include_annotation_keys",
			mcp.Description("List of annotation keys to include in output (if specified, only these are included)"),
			mcp.Items(map[string]any{
				"type": "string",
			}),
		),
		mcp.WithInteger("limit",
			mcp.Description("Maximum number of resources to return per page (1-500). "+
				"0 or omitted returns up to 500; use the returned continue token "+
				"to fetch the next page."),
			mcp.Min(0),
			mcp.Max(500)),
		mcp.WithString("continue",
			mcp.Description("Continue token for pagination (use the value from previous response's ListMeta.Continue)")),
		mcp.WithToolAnnotation(readOnlyAnnotations(listResourcesTitle)),
	)
}

// NewApplyResourceTool creates a new apply_resource tool
func NewApplyResourceTool() mcp.Tool {
	return mcp.NewTool(types.ApplyResourceToolName,
		mcp.WithToolTitle(applyResourceTitle),
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
		// Apply can overwrite fields of an existing resource, so it's
		// destructive. Re-applying the same manifest has no further effect.
		mcp.WithToolAnnotation(writeAnnotations(applyResourceTitle, true)),
	)
}

// NewClusteredResourceTemplate creates a new clustered resource template
func NewClusteredResourceTemplate() mcp.ResourceTemplate {
	return mcp.NewResourceTemplate(
		"k8s://clustered/{group}/{version}/{resource}/{name}",
		"Kubernetes Clustered Resource",
		mcp.WithTemplateDescription("A Kubernetes clustered resource"),
		mcp.WithTemplateMIMEType(mimeTypeJSON),
	)
}

// NewNamespacedResourceTemplate creates a new namespaced resource template
func NewNamespacedResourceTemplate() mcp.ResourceTemplate {
	return mcp.NewResourceTemplate(
		"k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}",
		"Kubernetes Namespaced Resource",
		mcp.WithTemplateDescription("A Kubernetes namespaced resource"),
		mcp.WithTemplateMIMEType(mimeTypeJSON),
	)
}
