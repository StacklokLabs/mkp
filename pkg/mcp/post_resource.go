package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandlePostResource handles the post_resource tool
func (m *Implementation) HandlePostResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")
	name := mcp.ParseString(request, "name", "")
	subresource := mcp.ParseString(request, "subresource", "")
	bodyMap := mcp.ParseStringMap(request, "body", nil)
	
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
	if resourceType == "namespaced" && namespace == "" {
		return mcp.NewToolResultError("namespace is required for namespaced resources"), nil
	}
	if bodyMap == nil {
		return mcp.NewToolResultError("body is required"), nil
	}

	// Create GVR
	// Validate resource_type
	if resourceType != "clustered" && resourceType != "namespaced" {
		return mcp.NewToolResultError("Invalid resource_type: " + resourceType), nil
	}

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Post to resource
	result, err := m.k8sClient.PostResource(ctx, gvr, namespace, name, subresource, bodyMap, parameters)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to post to resource", err), nil
	}

	// Convert to JSON
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to marshal result", err), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

// NewPostResourceTool creates a new post_resource tool
func NewPostResourceTool() mcp.Tool {
	return mcp.NewTool("post_resource",
		mcp.WithDescription("Post to a Kubernetes resource or its subresource"),
		mcp.WithString("resource_type",
			mcp.Description("Type of resource to post to (clustered or namespaced)"),
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
			mcp.Description("Name of the resource to post to"),
			mcp.Required()),
		mcp.WithString("subresource",
			mcp.Description("Subresource to post to (e.g., exec)")),
		mcp.WithObject("body",
			mcp.Description("Body to post to the resource. For pod exec, include 'command' (string or array), 'container' (optional), and 'timeout' (optional, in seconds)"),
			mcp.Required()),
		mcp.WithObject("parameters",
			mcp.Description("Optional parameters for the request")),
	)
}