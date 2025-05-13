package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleGetResource handles the get_resource tool
//
//nolint:gocyclo // This is deemed a complex function, but realistically it's not too bad
func (m *Implementation) HandleGetResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")
	name := mcp.ParseString(request, "name", "")
	subresource := mcp.ParseString(request, "subresource", "")

	// Parse parameters for subresources
	var parameters map[string]string
	if paramsRaw, exists := request.Params.Arguments["parameters"]; exists && paramsRaw != nil {
		if paramsMap, ok := paramsRaw.(map[string]interface{}); ok {
			parameters = make(map[string]string)
			for k, v := range paramsMap {
				if strVal, ok := v.(string); ok {
					parameters[k] = strVal
				} else {
					parameters[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

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
	if resourceType != "clustered" && resourceType != ResourceTypeNamespaced {
		return mcp.NewToolResultError("Invalid resource_type: " + resourceType), nil
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Get resource
	result, err := m.k8sClient.GetResource(ctx, gvr, namespace, name, subresource, parameters)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to get resource", err), nil
	}

	// Convert to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to marshal result", err), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// NewGetResourceTool creates a new get_resource tool
func NewGetResourceTool() mcp.Tool {
	return mcp.NewTool("get_resource",
		mcp.WithDescription("Get a Kubernetes resource or its subresource"),
		mcp.WithString("resource_type",
			mcp.Description("Type of resource to get (clustered or namespaced)"),
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
			mcp.Description("Name of the resource to get"),
			mcp.Required()),
		mcp.WithString("subresource",
			mcp.Description("Subresource to get (e.g., status, scale, logs)")),
		mcp.WithObject("parameters",
			mcp.Description(`Optional parameters for the request. For regular resources: resourceVersion. 
			For pod logs: container, previous, sinceSeconds, sinceTime, timestamps, limitBytes, tailLines`)),
		mcp.WithToolAnnotation(mcp.ToolAnnotation{
			Title:        "Get Kubernetes resource",
			ReadOnlyHint: BoolPtr(true),
		}),
	)
}
