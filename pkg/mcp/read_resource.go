package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ResourceURIComponents represents the components of a Kubernetes resource URI
type ResourceURIComponents struct {
	Group     string
	Version   string
	Resource  string
	Name      string
	Namespace string // Only for namespaced resources
}

// parseURI parses a URI and returns the parts after the prefix
// It handles empty parts (double slashes) by filtering them out
func parseURI(uri, prefix string) ([]string, error) {
	// Check prefix
	if !strings.HasPrefix(uri, prefix) {
		return nil, fmt.Errorf("invalid URI format: missing prefix %s", prefix)
	}

	// Get the path after the prefix
	path := uri[len(prefix):]

	// Split the path
	parts := strings.Split(path, "/")

	// Filter out empty parts (handles double slashes for empty group)
	filteredParts := []string{}
	for _, part := range parts {
		if part != "" {
			filteredParts = append(filteredParts, part)
		}
	}

	return filteredParts, nil
}

// parseClusteredResourceURI parses a clustered resource URI
// URI format: k8s://clustered/{group}/{version}/{resource}/{name}
// For core API group, the group can be empty (represented by a double slash)
func parseClusteredResourceURI(uri string) (ResourceURIComponents, error) {
	// Parse the URI
	parts, err := parseURI(uri, "k8s://clustered/")
	if err != nil {
		return ResourceURIComponents{}, err
	}

	// Check if we have enough parts
	if len(parts) < 3 {
		return ResourceURIComponents{}, fmt.Errorf("invalid URI format: expected at least 3 parts after prefix, got %d", len(parts))
	}

	components := ResourceURIComponents{}

	// Handle the case where the group is empty (core API group)
	if len(parts) == 3 {
		// Assume the group is empty (core API group)
		components.Group = ""
		components.Version = parts[0]
		components.Resource = parts[1]
		components.Name = parts[2]
	} else {
		// Normal case with a group
		components.Group = parts[0]
		components.Version = parts[1]
		components.Resource = parts[2]
		components.Name = parts[3]
	}

	return components, nil
}

// parseNamespacedResourceURI parses a namespaced resource URI
// URI format: k8s://namespaced/{namespace}/{group}/{version}/{resource}/{name}
// For core API group, the group can be empty (represented by a double slash)
func parseNamespacedResourceURI(uri string) (ResourceURIComponents, error) {
	// Parse the URI
	parts, err := parseURI(uri, "k8s://namespaced/")
	if err != nil {
		return ResourceURIComponents{}, err
	}

	// Check if we have enough parts
	if len(parts) < 4 {
		return ResourceURIComponents{}, fmt.Errorf("invalid URI format: expected at least 4 parts after prefix, got %d", len(parts))
	}

	components := ResourceURIComponents{}
	components.Namespace = parts[0]

	// Handle the case where the group is empty (core API group)
	if len(parts) == 4 {
		// Assume the group is empty (core API group)
		components.Group = ""
		components.Version = parts[1]
		components.Resource = parts[2]
		components.Name = parts[3]
	} else {
		// Normal case with a group
		components.Group = parts[1]
		components.Version = parts[2]
		components.Resource = parts[3]
		components.Name = parts[4]
	}

	return components, nil
}

// HandleClusteredResource handles the clustered resource template
func (m *Implementation) HandleClusteredResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Parse the URI
	components, err := parseClusteredResourceURI(request.Params.URI)
	if err != nil {
		return nil, err
	}

	// Create GVR
	gvr := schema.GroupVersionResource{
		Group:    components.Group,
		Version:  components.Version,
		Resource: components.Resource,
	}

	// Get resource
	obj, err := m.k8sClient.GetClusteredResource(ctx, gvr, components.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Convert to JSON
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(result),
		},
	}, nil
}

// HandleNamespacedResource handles the namespaced resource template
func (m *Implementation) HandleNamespacedResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Parse the URI
	components, err := parseNamespacedResourceURI(request.Params.URI)
	if err != nil {
		return nil, err
	}

	// Create GVR
	gvr := schema.GroupVersionResource{
		Group:    components.Group,
		Version:  components.Version,
		Resource: components.Resource,
	}

	// Get resource
	obj, err := m.k8sClient.GetNamespacedResource(ctx, gvr, components.Namespace, components.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Convert to JSON
	result, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(result),
		},
	}, nil
}
