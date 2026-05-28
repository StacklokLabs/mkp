package mcp

// Resource scope identifiers accepted by tools that operate on
// either cluster-scoped or namespaced Kubernetes resources.
const (
	resourceTypeClustered  = "clustered"
	resourceTypeNamespaced = "namespaced"
)

// MIME types and well-known annotation keys reused across tool/resource
// definitions.
const (
	mimeTypeJSON = "application/json"

	// kubectlLastAppliedAnnotation is excluded from default tool output
	// because it duplicates the manifest body and bloats LLM context.
	kubectlLastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"
)
