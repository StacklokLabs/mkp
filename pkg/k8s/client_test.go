package k8s

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestListClusteredResources(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}

	// Add a fake list response
	client.PrependReactor("list", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]interface{}{
							"name": "test-cluster-role",
						},
						"rules": []interface{}{
							map[string]interface{}{
								"apiGroups": []interface{}{""},
								"resources": []interface{}{"pods"},
								"verbs":     []interface{}{"get", "list", "watch"},
							},
						},
					},
				},
				{
					Object: map[string]interface{}{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]interface{}{
							"name": "test-cluster-role-2",
							"labels": map[string]interface{}{
								"app": "test-app",
							},
						},
						"rules": []interface{}{
							map[string]interface{}{
								"apiGroups": []interface{}{""},
								"resources": []interface{}{"pods"},
								"verbs":     []interface{}{"get", "list", "watch"},
							},
						},
					},
				},
			},
		}
		return true, list, nil
	})

	// Test ListClusteredResources
	ctx := context.Background()
	list, err := testClient.ListClusteredResources(ctx, gvr, "", 0, "")

	// Verify there was no error
	assert.NoError(t, err, "ListClusteredResources should not return an error")

	// Verify the result
	assert.Len(t, list.Items, 2, "Expected 2 items")
	assert.Equal(t, "test-cluster-role", list.Items[0].GetName(), "Expected name 'test-cluster-role'")

	// Test ListClusteredResources with label selector
	list, err = testClient.ListClusteredResources(ctx, gvr, "app=test-app", 0, "")

	// Verify there was no error
	assert.NoError(t, err, "ListClusteredResources should not return an error")

	// Verify the result
	assert.Len(t, list.Items, 1, "Expected 1 item")
	assert.Equal(t, "test-cluster-role-2", list.Items[0].GetName(), "Expected name 'test-cluster-role-2'")
}

func TestListNamespacedResources(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()

	// Register list kinds for the resources we'll be testing
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "services"}: "ServiceList",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	// Add a fake list response
	client.PrependReactor("list", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Items: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name":      "test-service",
							"namespace": "default",
						},
						"spec": map[string]interface{}{
							"ports": []interface{}{
								map[string]interface{}{
									"port":     int64(80),
									"protocol": "TCP",
								},
							},
						},
					},
				},
				{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name":      "test-service-2",
							"namespace": "default",
							"labels": map[string]interface{}{
								"app": "test-app",
							},
						},
						"spec": map[string]interface{}{
							"ports": []interface{}{
								map[string]interface{}{
									"port":     int64(80),
									"protocol": "TCP",
								},
							},
						},
					},
				},
			},
		}
		return true, list, nil
	})

	// Test ListNamespacedResources
	ctx := context.Background()
	list, err := testClient.ListNamespacedResources(ctx, gvr, "default", "", 0, "")

	// Verify there was no error
	assert.NoError(t, err, "ListNamespacedResources should not return an error")

	// Verify the result
	assert.Len(t, list.Items, 2, "Expected 2 items")
	assert.Equal(t, "test-service", list.Items[0].GetName(), "Expected name 'test-service'")

	// Test ListNamespacedResources with label selector
	list, err = testClient.ListNamespacedResources(ctx, gvr, "default", "app=test-app", 0, "")

	// Verify there was no error
	assert.NoError(t, err, "ListNamespacedResources should not return an error")

	// Verify the result
	assert.Len(t, list.Items, 1, "Expected 1 item")
	assert.Equal(t, "test-service-2", list.Items[0].GetName(), "Expected name 'test-service-2'")
}

// TestListClusteredResourcesWithPagination tests that pagination parameters are correctly handled
func TestListClusteredResourcesWithPagination(t *testing.T) {
	// Since the fake client doesn't properly pass through ListOptions in the reactor,
	// we'll test that our functions correctly build the ListOptions and that the
	// response's continue token is properly preserved

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	testClient := &Client{
		dynamicClient: client,
	}

	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}

	// Add a reactor that returns a list with a continue token
	client.PrependReactor("list", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"continue": "test-continue-token",
				},
			},
			Items: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]interface{}{
							"name": "test-role",
						},
					},
				},
			},
		}
		return true, list, nil
	})

	// Test that the function accepts pagination parameters and returns results
	ctx := context.Background()

	// Test with limit
	list, err := testClient.ListClusteredResources(ctx, gvr, "", 10, "")
	assert.NoError(t, err, "ListClusteredResources with limit should not return an error")
	assert.NotNil(t, list, "List should not be nil")
	assert.Equal(t, "test-continue-token", list.GetContinue(), "Continue token should be preserved in response")

	// Test with continue token
	list, err = testClient.ListClusteredResources(ctx, gvr, "", 0, "my-continue-token")
	assert.NoError(t, err, "ListClusteredResources with continue token should not return an error")
	assert.NotNil(t, list, "List should not be nil")
}

// TestListNamespacedResourcesWithPagination tests that pagination parameters are correctly handled
func TestListNamespacedResourcesWithPagination(t *testing.T) {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}: "PodList",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	testClient := &Client{
		dynamicClient: client,
	}

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// Add a reactor that returns a list with a continue token
	client.PrependReactor("list", "pods", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"continue": "pod-continue-token",
				},
			},
			Items: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Pod",
						"metadata": map[string]interface{}{
							"name":      "test-pod",
							"namespace": "default",
						},
					},
				},
			},
		}
		return true, list, nil
	})

	// Test that the function accepts pagination parameters and returns results
	ctx := context.Background()

	// Test with limit
	list, err := testClient.ListNamespacedResources(ctx, gvr, "default", "", 5, "")
	assert.NoError(t, err, "ListNamespacedResources with limit should not return an error")
	assert.NotNil(t, list, "List should not be nil")
	assert.Equal(t, "pod-continue-token", list.GetContinue(), "Continue token should be preserved in response")

	// Test with continue token
	list, err = testClient.ListNamespacedResources(ctx, gvr, "default", "", 0, "pod-token")
	assert.NoError(t, err, "ListNamespacedResources with continue token should not return an error")
	assert.NotNil(t, list, "List should not be nil")
}

func TestApplyClusteredResource(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
			},
			"rules": []interface{}{
				map[string]interface{}{
					"apiGroups": []interface{}{""},
					"resources": []interface{}{"pods"},
					"verbs":     []interface{}{"get", "list", "watch"},
				},
			},
		},
	}

	// Add a fake get response (resource not found)
	client.PrependReactor("get", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: clusterroles \"test-cluster-role\" not found")
	})

	// Add a fake create response
	client.PrependReactor("create", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Test ApplyClusteredResource
	ctx := context.Background()
	result, err := testClient.ApplyClusteredResource(ctx, gvr, obj)

	// Verify there was no error
	assert.NoError(t, err, "ApplyClusteredResource should not return an error")

	// Verify the result
	assert.Equal(t, "test-cluster-role", result.GetName(), "Expected name 'test-cluster-role'")

}

func TestApplyNamespacedResource(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
		},
	}

	// Add a fake get response (resource not found)
	client.PrependReactor("get", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("not found: services \"test-service\" not found")
	})

	// Add a fake create response
	client.PrependReactor("create", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Test ApplyNamespacedResource
	ctx := context.Background()
	result, err := testClient.ApplyNamespacedResource(ctx, gvr, "default", obj)

	// Verify there was no error
	assert.NoError(t, err, "ApplyNamespacedResource should not return an error")

	// Verify the result
	assert.Equal(t, "test-service", result.GetName(), "Expected name 'test-service'")
}

func TestGetClusteredResource(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name": "test-cluster-role",
			},
			"rules": []interface{}{
				map[string]interface{}{
					"apiGroups": []interface{}{""},
					"resources": []interface{}{"pods"},
					"verbs":     []interface{}{"get", "list", "watch"},
				},
			},
		},
	}

	// Add a fake get response
	client.PrependReactor("get", "clusterroles", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Test GetClusteredResource
	ctx := context.Background()
	result, err := testClient.GetClusteredResource(ctx, gvr, "test-cluster-role")

	// Verify there was no error
	assert.NoError(t, err, "GetClusteredResource should not return an error")

	// Verify the result
	unstructuredResult, ok := result.(*unstructured.Unstructured)
	assert.True(t, ok, "Expected *unstructured.Unstructured")
	assert.Equal(t, "test-cluster-role", unstructuredResult.GetName(), "Expected name 'test-cluster-role'")
}

func TestGetNamespacedResource(t *testing.T) {
	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create a test client with the fake dynamic client
	testClient := &Client{
		dynamicClient: client,
	}

	// Create a test GVR
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	// Create a test resource
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      "test-service",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"port":     int64(80),
						"protocol": "TCP",
					},
				},
			},
		},
	}

	// Add a fake get response
	client.PrependReactor("get", "services", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	// Test GetNamespacedResource
	ctx := context.Background()
	result, err := testClient.GetNamespacedResource(ctx, gvr, "default", "test-service")

	// Verify there was no error
	assert.NoError(t, err, "GetNamespacedResource should not return an error")

	// Verify the result
	unstructuredResult, ok := result.(*unstructured.Unstructured)
	assert.True(t, ok, "Expected *unstructured.Unstructured")
	assert.Equal(t, "test-service", unstructuredResult.GetName(), "Expected name 'test-service'")
}

func TestSetDynamicClient(t *testing.T) {
	// Create a test client
	testClient := &Client{}

	// Create a fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Set the dynamic client
	testClient.SetDynamicClient(fakeDynamicClient)

	// Verify the dynamic client was set
	assert.NotNil(t, testClient.dynamicClient, "Expected dynamicClient to be set")
}

func TestSetDiscoveryClient(t *testing.T) {
	// Create a test client
	testClient := &Client{}

	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}

	// Set the discovery client
	testClient.SetDiscoveryClient(fakeDiscoveryClient)

	// Verify the discovery client was set
	assert.NotNil(t, testClient.discoveryClient, "Expected discoveryClient to be set")
}

func TestIsReady(t *testing.T) {
	// Test with all clients nil
	testClient := &Client{}
	assert.False(t, testClient.IsReady(), "Expected IsReady to return false when all clients are nil")

	// Test with only dynamic client set
	testClient = &Client{}
	scheme := runtime.NewScheme()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)
	testClient.SetDynamicClient(fakeDynamicClient)
	assert.False(t, testClient.IsReady(), "Expected IsReady to return false when some clients are nil")

	// Test with only discovery client set
	testClient = &Client{}
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}
	testClient.SetDiscoveryClient(fakeDiscoveryClient)
	assert.False(t, testClient.IsReady(), "Expected IsReady to return false when some clients are nil")

	// Test with only clientset set
	testClient = &Client{}
	fakeClientset := kubefake.NewSimpleClientset()
	testClient.SetClientset(fakeClientset)
	assert.False(t, testClient.IsReady(), "Expected IsReady to return false when some clients are nil")

	// Test with all clients set
	testClient = &Client{}
	testClient.SetDynamicClient(fakeDynamicClient)
	testClient.SetDiscoveryClient(fakeDiscoveryClient)
	testClient.SetClientset(fakeClientset)
	assert.True(t, testClient.IsReady(), "Expected IsReady to return true when all clients are set")
}

func TestListAPIResources(t *testing.T) {
	// Create a fake discovery client
	fakeDiscoveryClient := &discoveryfake.FakeDiscovery{Fake: &ktesting.Fake{}}

	// Add some fake API resources
	fakeDiscoveryClient.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Kind:       "Pod",
					Namespaced: true,
				},
				{
					Name:       "services",
					Kind:       "Service",
					Namespaced: true,
				},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "deployments",
					Kind:       "Deployment",
					Namespaced: true,
				},
				{
					Name:       "statefulsets",
					Kind:       "StatefulSet",
					Namespaced: true,
				},
			},
		},
	}

	// Create a test client with the fake discovery client
	testClient := &Client{
		discoveryClient: fakeDiscoveryClient,
	}

	// Test ListAPIResources
	ctx := context.Background()
	resources, err := testClient.ListAPIResources(ctx)

	// Verify there was no error
	assert.NoError(t, err, "ListAPIResources should not return an error")

	// Verify the result
	assert.Len(t, resources, 2, "Expected 2 resource lists")

	// Check the first resource list
	assert.Equal(t, "v1", resources[0].GroupVersion, "Expected GroupVersion 'v1'")
	assert.Len(t, resources[0].APIResources, 2, "Expected 2 API resources in the first list")

	// Check the second resource list
	assert.Equal(t, "apps/v1", resources[1].GroupVersion, "Expected GroupVersion 'apps/v1'")
	assert.Len(t, resources[1].APIResources, 2, "Expected 2 API resources in the second list")
}

func TestEffectiveListLimit(t *testing.T) {
	// The cap is the security property for GHSA-pcvx-mc89-xfjm: an omitted
	// (0) or over-cap limit must never produce an unbounded list request.
	tests := []struct {
		name      string
		requested int64
		want      int64
	}{
		{"omitted defaults to cap", 0, maxListLimit},
		{"negative defaults to cap", -1, maxListLimit},
		{"below cap is honoured", 10, 10},
		{"one below cap is honoured", maxListLimit - 1, maxListLimit - 1},
		{"at cap is clamped to cap", maxListLimit, maxListLimit},
		{"above cap is clamped to cap", maxListLimit + 1, maxListLimit},
		{"far above cap is clamped to cap", 1_000_000, maxListLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, effectiveListLimit(tt.requested),
				"effectiveListLimit(%d)", tt.requested)
			assert.LessOrEqual(t, effectiveListLimit(tt.requested), maxListLimit,
				"effective limit must never exceed the cap")
			assert.Positive(t, effectiveListLimit(tt.requested),
				"effective limit must always be a bounded, positive page size")
		})
	}
}

func TestListResourcesAppliesCap(t *testing.T) {
	// End-to-end: a list with limit omitted must reach the API with a bounded
	// Limit, not the zero value that means "return everything". The fake
	// dynamic client exposes the ListOptions it was called with via the
	// recorded action, which we inspect in the reactor.
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	testClient := &Client{
		dynamicClient: client,
		listReadSem:   make(chan struct{}, maxConcurrentListReads),
	}
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	var gotLimit int64 = -1
	client.PrependReactor("list", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
		if la, ok := action.(ktesting.ListActionImpl); ok {
			gotLimit = la.ListOptions.Limit
		}
		return true, &unstructured.UnstructuredList{}, nil
	})

	// limit omitted (0): must be sent as the cap, never 0/unbounded.
	_, err := testClient.ListNamespacedResources(context.Background(), gvr, "kube-system", "", 0, "")
	assert.NoError(t, err)
	assert.Equal(t, maxListLimit, gotLimit, "omitted limit must be sent as the cap, not unbounded")

	// over-cap limit: must be clamped down.
	gotLimit = -1
	_, err = testClient.ListNamespacedResources(context.Background(), gvr, "kube-system", "", 100_000, "")
	assert.NoError(t, err)
	assert.Equal(t, maxListLimit, gotLimit, "over-cap limit must be clamped to the cap")
}

func TestAcquireListSlotRespectsContextCancellation(t *testing.T) {
	// A full semaphore must not block forever: a cancelled context releases
	// the waiter so a slow client cannot pin the list path.
	c := &Client{listReadSem: make(chan struct{}, 1)}

	release, err := c.acquireListSlot(context.Background())
	assert.NoError(t, err, "first slot should be acquired immediately")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.acquireListSlot(ctx)
	assert.Error(t, err, "acquiring a slot on a full semaphore with a cancelled ctx should fail")

	// Releasing the first slot lets the next acquire succeed.
	release()
	release2, err := c.acquireListSlot(context.Background())
	assert.NoError(t, err, "slot should be acquirable again after release")
	release2()
}

func TestAcquireListSlotNilSemaphore(t *testing.T) {
	// A client built without a semaphore (e.g. a bare test client) must still
	// work: acquireListSlot returns a no-op release.
	c := &Client{}
	release, err := c.acquireListSlot(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, release)
	release() // must not panic
}
