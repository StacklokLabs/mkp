package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/StacklokLabs/mkp/pkg/types"
)

// HandlePostResource handles the post_resource tool
func (m *Implementation) HandlePostResource(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse and validate parameters
	params, err := parsePostResourceParams(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Create GVR
	gvr := schema.GroupVersionResource{
		Group:    params.group,
		Version:  params.version,
		Resource: params.resource,
	}

	// Post to resource
	result, err := m.k8sClient.PostResource(
		ctx, gvr, params.namespace, params.name,
		params.subresource, params.bodyMap, params.parameters,
	)
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

// postResourceParams holds the parsed and validated parameters for the post_resource tool
type postResourceParams struct {
	resourceType string
	group        string
	version      string
	resource     string
	namespace    string
	name         string
	subresource  string
	bodyMap      map[string]interface{}
	parameters   map[string]string
}

// parsePostResourceParams parses and validates the parameters for the post_resource tool
func parsePostResourceParams(request mcp.CallToolRequest) (*postResourceParams, error) {
	params := &postResourceParams{
		resourceType: mcp.ParseString(request, "resource_type", ""),
		group:        mcp.ParseString(request, "group", ""),
		version:      mcp.ParseString(request, "version", ""),
		resource:     mcp.ParseString(request, "resource", ""),
		namespace:    mcp.ParseString(request, "namespace", ""),
		name:         mcp.ParseString(request, "name", ""),
		subresource:  mcp.ParseString(request, "subresource", ""),
		bodyMap:      mcp.ParseStringMap(request, "body", nil),
	}

	// Parse parameters for subresources
	if paramsRaw, exists := request.Params.Arguments["parameters"]; exists && paramsRaw != nil {
		if paramsMap, ok := paramsRaw.(map[string]interface{}); ok {
			params.parameters = make(map[string]string)
			for k, v := range paramsMap {
				if strVal, ok := v.(string); ok {
					params.parameters[k] = strVal
				} else {
					params.parameters[k] = fmt.Sprintf("%v", v)
				}
			}
		}
	}

	// Validate parameters
	if params.resourceType == "" {
		return nil, fmt.Errorf("resource_type is required")
	}
	if params.version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if params.resource == "" {
		return nil, fmt.Errorf("resource is required")
	}
	if params.name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if params.resourceType == "namespaced" && params.namespace == "" {
		return nil, fmt.Errorf("namespace is required for namespaced resources")
	}
	if params.bodyMap == nil {
		return nil, fmt.Errorf("body is required")
	}

	// Validate resource_type
	if params.resourceType != "clustered" && params.resourceType != "namespaced" {
		return nil, fmt.Errorf("invalid resource_type: %s", params.resourceType)
	}

	return params, nil
}

// NewPostResourceTool creates a new post_resource tool
func NewPostResourceTool() mcp.Tool {
	return mcp.NewTool(types.PostResourceToolName,
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
			mcp.Description("Body to post to the resource. For pod exec, include 'command' (string or array), "+
				"'container' (optional), and 'timeout' (optional, in seconds)"),
			mcp.Required()),
		mcp.WithObject("parameters",
			mcp.Description("Optional parameters for the request")),
	)
}
