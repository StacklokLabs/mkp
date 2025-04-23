package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleListResources handles the list_resources tool
func (m *Implementation) HandleListResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")

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
	if resourceType == "namespaced" && namespace == "" {
		return mcp.NewToolResultError("namespace is required for namespaced resources"), nil
	}

	// Create GVR
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// List resources
	var list *unstructured.UnstructuredList
	var err error
	switch resourceType {
	case "clustered":
		list, err = m.k8sClient.ListClusteredResources(ctx, gvr)
	case "namespaced":
		list, err = m.k8sClient.ListNamespacedResources(ctx, gvr, namespace)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid resource_type: %s", resourceType)), nil
	}

	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to list resources", err), nil
	}

	// Convert to JSON
	result, err := json.Marshal(list)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to marshal result", err), nil
	}

	return mcp.NewToolResultText(string(result)), nil
}

// HandleListAllResources handles the listResources request
func (m *Implementation) HandleListAllResources(ctx context.Context) ([]mcp.Resource, error) {
	// Get API resources from Kubernetes
	apiResources, err := m.k8sClient.ListAPIResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list API resources: %w", err)
	}

	// Convert API resources to MCP resources
	resources := []mcp.Resource{}
	for _, resourceList := range apiResources {
		gv := strings.Split(resourceList.GroupVersion, "/")
		var group, version string
		if len(gv) == 1 {
			// Core API group has no prefix
			group = ""
			version = gv[0]
		} else {
			group = gv[0]
			version = gv[1]
		}

		for _, apiResource := range resourceList.APIResources {
			// Skip subresources
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			// Create resource name
			var name string
			if apiResource.Namespaced {
				name = fmt.Sprintf("Namespaced %s", apiResource.Kind)
			} else {
				name = fmt.Sprintf("Clustered %s", apiResource.Kind)
			}

			// Create resource URI
			var uri string
			if apiResource.Namespaced {
				uri = fmt.Sprintf("k8s://namespaced/default/%s/%s/%s/example", group, version, apiResource.Name)
			} else {
				uri = fmt.Sprintf("k8s://clustered/%s/%s/%s/example", group, version, apiResource.Name)
			}

			// Create resource
			resource := mcp.NewResource(
				uri,
				name,
				mcp.WithResourceDescription(fmt.Sprintf("Kubernetes %s resource", apiResource.Kind)),
				mcp.WithMIMEType("application/json"),
			)

			resources = append(resources, resource)
		}
	}

	return resources, nil
}
