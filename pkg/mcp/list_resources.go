package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/StacklokLabs/mkp/pkg/types"
)

// HandleListResources handles the list_resources tool
// Parameters:
//   - resource_type: Type of the resource ("clustered" or "namespaced")
//   - group: API group of the resource
//   - version: API version of the resource
//   - resource: Resource type (e.g., "pods", "services")
//   - namespace: Namespace for namespaced resources
//   - label_selector: Kubernetes label selector for filtering resources (optional)
//     Label selector format: "key1=value1,key2=value2" for equality or "key1 in (value1, value2),!key3" for set-based
func (m *Implementation) HandleListResources(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse parameters
	resourceType := mcp.ParseString(request, "resource_type", "")
	group := mcp.ParseString(request, "group", "")
	version := mcp.ParseString(request, "version", "")
	resource := mcp.ParseString(request, "resource", "")
	namespace := mcp.ParseString(request, "namespace", "")
	labelSelector := mcp.ParseString(request, "label_selector", "")

	// Parse new annotation filtering parameters
	includeAnnotations := request.GetBool("include_annotations", true)
	excludeAnnotationKeys := request.GetStringSlice(
		"exclude_annotation_keys", []string{"kubectl.kubernetes.io/last-applied-configuration"})
	includeAnnotationKeys := request.GetStringSlice("include_annotation_keys", []string{})

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
	if labelSelector != "" {
		_, err := labels.Parse(labelSelector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid label selector: %v", err)), nil
		}
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
	case types.ResourceTypeClustered:
		list, err = m.k8sClient.ListClusteredResources(ctx, gvr, labelSelector)
	case types.ResourceTypeNamespaced:
		list, err = m.k8sClient.ListNamespacedResources(ctx, gvr, namespace, labelSelector)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid resource_type: %s", resourceType)), nil
	}

	if err != nil {
		return mcp.NewToolResultErrorFromErr("Failed to list resources", err), nil
	}

	// Convert to PartialObjectMetadataList
	metadataList := &metav1.PartialObjectMetadataList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PartialObjectMetadataList",
			APIVersion: "meta.k8s.io/v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: list.GetResourceVersion(),
		},
		Items: make([]metav1.PartialObjectMetadata, 0, len(list.Items)),
	}

	// Extract metadata from each resource
	for _, item := range list.Items {
		// Process annotations based on parameters
		var annotations map[string]string
		if includeAnnotations {
			rawAnnotations := item.GetAnnotations()
			if rawAnnotations != nil {
				annotations = filterAnnotations(rawAnnotations, includeAnnotationKeys, excludeAnnotationKeys)
			}
		}

		metadata := metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				Kind:       item.GetKind(),
				APIVersion: item.GetAPIVersion(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:              item.GetName(),
				Namespace:         item.GetNamespace(),
				Labels:            item.GetLabels(),
				Annotations:       annotations,
				ResourceVersion:   item.GetResourceVersion(),
				UID:               item.GetUID(),
				CreationTimestamp: item.GetCreationTimestamp(),
			},
		}

		metadataList.Items = append(metadataList.Items, metadata)
	}

	// Convert to JSON
	result, err := json.Marshal(metadataList)
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
