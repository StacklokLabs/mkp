// Package mcp implements the Model, Channel, Prompt (MCP) protocol services
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/StacklokLabs/mkp/pkg/types"
)

// HandleApplyResource handles the apply_resource tool
func (m *Implementation) HandleApplyResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")
	manifestMap := mcp.ParseStringMap(request, "manifest", nil)

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
	if resourceType == types.ResourceTypeNamespaced && namespace == "" {
		return mcp.NewToolResultError("namespace is required for namespaced resources"), nil
	}
	if manifestMap == nil {
		return mcp.NewToolResultError("manifest is required"), nil
	}

	// Create GVR
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Get the appropriate client (may be impersonated)
	client, clientErr := m.clientForContext(ctx)
	if clientErr != nil {
		return mcp.NewToolResultErrorFromErr("Failed to get Kubernetes client", clientErr), nil
	}

	// Convert manifest to unstructured
	obj := &unstructured.Unstructured{Object: manifestMap}

	// Apply resource
	var result *unstructured.Unstructured
	var err error
	switch resourceType {
	case types.ResourceTypeClustered:
		result, err = client.ApplyClusteredResource(ctx, gvr, obj)
	case types.ResourceTypeNamespaced:
		result, err = client.ApplyNamespacedResource(ctx, gvr, namespace, obj)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid resource_type: %s", resourceType)), nil
	}

	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to apply resource", err), nil
	}

	// Convert to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to marshal result", err), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}
