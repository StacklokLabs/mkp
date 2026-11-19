package mcp

import "github.com/mark3labs/mcp-go/mcp"

// Human-readable tool titles, used both as the top-level tool title and in
// the tool annotations.
const (
	listResourcesTitle  = "List Kubernetes Resources"
	getResourceTitle    = "Get Kubernetes Resource"
	applyResourceTitle  = "Apply Kubernetes Resource"
	deleteResourceTitle = "Delete Kubernetes Resource"
	postResourceTitle   = "Post to Kubernetes Resource"
)

// Every hint is set explicitly. mcp.WithToolAnnotation replaces the whole
// annotation struct, so any hint left nil is omitted from the tool listing
// and clients fall back to the spec defaults (destructiveHint=true,
// openWorldHint=true) or treat the tool as unannotated.
//
// openWorldHint is false for every tool: each one acts only on the
// configured Kubernetes cluster, which is a closed domain.

// readOnlyAnnotations returns annotations for a tool that only reads from
// the cluster.
func readOnlyAnnotations(title string) mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    BoolPtr(true),
		DestructiveHint: BoolPtr(false),
		IdempotentHint:  BoolPtr(true),
		OpenWorldHint:   BoolPtr(false),
	}
}

// writeAnnotations returns annotations for a tool that can modify or remove
// existing cluster state.
func writeAnnotations(title string, idempotent bool) mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    BoolPtr(false),
		DestructiveHint: BoolPtr(true),
		IdempotentHint:  BoolPtr(idempotent),
		OpenWorldHint:   BoolPtr(false),
	}
}
