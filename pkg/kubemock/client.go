// Package kubemock provides in-memory Kubernetes fakes for testing controllers,
// event-based listeners and stores without a running cluster.
//
// Two layers are offered:
//
//   - Small fakes for unit tests: NewFakeClient (client-go clientset with DryRun
//     support) and the mock listers.
//   - FakeCluster plus NewManager*: one object store shared by a controller-runtime
//     client, a dynamic client and a controller-runtime manager whose cache
//     delivers informer events.
//
// All fakes write through one tracker that mirrors API-server semantics:
//
//   - Generation: metadata.generation starts at 1 and only increments on
//     desired-state changes (fields outside metadata/status) and deletion start.
//   - No-op writes: an identical write keeps its resourceVersion and emits no
//     event, so status-writing reconcilers converge.
//   - Informer events reflect stored state: deleting an object with finalizers
//     is an update (deletionTimestamp set); removing the last finalizer is the
//     delete. Informers replay existing objects to new handlers.
//
// Custom resources with a Go type must register it and its List type in the
// scheme for typed List calls, as with a real controller-runtime client. Resources without a Go
// type, or cluster-scoped ones, are added with FakeCluster.InstallResource,
// the equivalent of applying a CRD. Writing an object whose kind the resource
// doesn't serve fails with BadRequest, as on a real API server.
//
// Usage:
//
//	scheme := runtime.NewScheme()
//	_ = corev1.AddToScheme(scheme)
//	cluster := kubemock.NewFakeCluster(scheme, existingObjects...)
//	mgr, err := kubemock.NewManagerForFakeCluster(nil, ctrl.Options{Scheme: scheme}, cluster)
//	// register controllers on mgr, start it, write via mgr.GetClient()
//	// or cluster.DynamicClient(); reconcilers observe the events.
package kubemock

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// FakeCluster holds one object tracker exposed through a controller-runtime
// client, a dynamic client and an informer cache, so a manager and a
// Kubernetes-backed store operate on the same in-memory state. Writes through
// Client or DynamicClient emit informer events; writes made directly on
// Tracker bypass them.
type FakeCluster struct {
	scheme  *runtime.Scheme
	mapper  *meta.DefaultRESTMapper
	tracker testing.ObjectTracker
	client  *eventingClient
	dynamic *dynamicClient
	cache   *fakeCache
}

// NewFakeCluster creates a FakeCluster for scheme (client-go's scheme when nil)
// seeded with initObjs. It panics if an initial object cannot be stored, as
// that is a test setup error.
func NewFakeCluster(scheme *runtime.Scheme, initObjs ...runtime.Object) *FakeCluster {
	if scheme == nil {
		scheme = clientgoscheme.Scheme
	}
	tracker := newGenerationTrackingObjectTracker(scheme)
	for _, obj := range initObjs {
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Sprintf("kubemock: failed to add initial object: %v", err))
		}
	}
	mapper := defaultRESTMapperForScheme(scheme)
	raw := newFakeClientBuilder(scheme, mapper, tracker).Build()
	cache := newFakeCache(raw)
	client := &eventingClient{Client: raw, cache: cache}
	return &FakeCluster{
		scheme:  scheme,
		mapper:  mapper,
		tracker: tracker,
		client:  client,
		dynamic: &dynamicClient{client: client, tracker: tracker},
		cache:   cache,
	}
}

// InstallResource makes a custom resource available, like applying its CRD
// to a real cluster. The dynamic client (and anything built on it, such as the
// Kubernetes store) can then use the resource without a Go type in the scheme.
// When gvk has no Go type, it is registered as unstructured in the cluster's
// scheme; when it has one, that type is kept. A missing List type is filled
// in with a generic list, so dynamic List works as on a real cluster; typed
// List calls still need the Go List type.
// The resource gets a status subresource and the given scope.
//
// InstallResource must be called before the cluster is used concurrently,
// typically during test setup and before starting a manager. It panics on a
// setup error.
func (c *FakeCluster) InstallResource(gvk schema.GroupVersionKind, resource string, namespaced bool) {
	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	switch {
	case !c.scheme.Recognizes(gvk):
		c.scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		if !c.scheme.Recognizes(listGVK) {
			c.scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
		}
	case !c.scheme.Recognizes(listGVK):
		c.scheme.AddKnownTypeWithName(listGVK, &objectList{})
	}

	scope := meta.RESTScopeRoot
	if namespaced {
		scope = meta.RESTScopeNamespace
	}
	c.mapper.AddSpecific(gvk, gvk.GroupVersion().WithResource(resource), gvk.GroupVersion().WithResource(strings.ToLower(gvk.Kind)), scope)

	// The controller-runtime fake fixes the kinds with a status subresource
	// when it is built, so rebuild it. State lives in the shared tracker.
	raw := newFakeClientBuilder(c.scheme, c.mapper, c.tracker).Build()
	c.client.Client = raw
	c.cache.client = raw
}

// objectList lists any typed objects. The tracker needs a List type to answer
// List; this one serves Go types registered without their own List type.
type objectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []runtime.Object `json:"items"`
}

// DeepCopyObject implements runtime.Object. The tracker already returns
// copies, so items are shared.
func (l *objectList) DeepCopyObject() runtime.Object {
	out := *l
	out.Items = slices.Clone(l.Items)
	return &out
}

// Client returns the controller-runtime client. Its writes emit informer events.
func (s *FakeCluster) Client() ctrlclient.Client {
	return s.client
}

// Tracker returns the underlying object tracker.
func (s *FakeCluster) Tracker() testing.ObjectTracker {
	return s.tracker
}

// DynamicClient returns a dynamic client backed by Client.
func (s *FakeCluster) DynamicClient() dynamic.Interface {
	return s.dynamic
}

// ResourceClient returns a dynamic client for gvr, scoped to namespace unless empty.
func (s *FakeCluster) ResourceClient(gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	resource := s.dynamic.Resource(gvr)
	if namespace == "" {
		return resource
	}
	return resource.Namespace(namespace)
}

// NewFakeClient returns a client-go fake clientset with DryRun support and the
// package's generation and no-op semantics.
func NewFakeClient() *fake.Clientset {
	client := fake.NewSimpleClientset()
	tracker := &generationTrackingObjectTracker{ObjectTracker: client.Tracker()}
	client.PrependReactor("*", "*", testing.ObjectReaction(tracker))
	client.PrependReactor("*", "*", DryRunReactor(tracker))
	return client
}

// NewFakeControllerRuntimeClient creates a controller-runtime client backed by the same fake
// object tracker used by client-go's fake clientset. This lets tests build a manager using the
// standard controller-runtime client.Client contract without reimplementing CRUD logic by hand.
func NewFakeControllerRuntimeClient(initObjs ...runtime.Object) ctrlclient.Client {
	return NewControllerRuntimeClientFromFakeClient(NewFakeClient(), nil, nil, initObjs...)
}

// NewControllerRuntimeClientFromFakeClient creates a controller-runtime client using a client-go
// fake clientset as the backing object tracker. The client does not emit informer events; use
// FakeCluster or NewManagerWithClient for that.
func NewControllerRuntimeClientFromFakeClient(cs *fake.Clientset, scheme *runtime.Scheme, restMapper meta.RESTMapper, initObjs ...runtime.Object) ctrlclient.Client {
	if cs == nil {
		cs = NewFakeClient()
	}
	if scheme == nil {
		scheme = clientgoscheme.Scheme
	}
	if restMapper == nil {
		restMapper = defaultRESTMapperForScheme(scheme)
	}
	tracker := &generationTrackingObjectTracker{ObjectTracker: cs.Tracker()}
	return newFakeClientBuilder(scheme, restMapper, tracker).WithRuntimeObjects(initObjs...).Build()
}

// newFakeClientBuilder enables the status subresource for every object kind,
// matching CRDs with a status subresource and most built-in types.
func newFakeClientBuilder(scheme *runtime.Scheme, mapper meta.RESTMapper, tracker testing.ObjectTracker) *ctrlfake.ClientBuilder {
	builder := ctrlfake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjectTracker(tracker)
	forEachObjectKind(scheme, func(gvk schema.GroupVersionKind, obj ctrlclient.Object) {
		builder = builder.WithStatusSubresource(obj)
	})
	return builder
}

// forEachObjectKind calls fn for every external, non-list kind in scheme
// whose type is a client.Object.
func forEachObjectKind(scheme *runtime.Scheme, fn func(schema.GroupVersionKind, ctrlclient.Object)) {
	for gvk := range scheme.AllKnownTypes() {
		if gvk.Version == runtime.APIVersionInternal || strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		obj, err := scheme.New(gvk)
		if err != nil {
			continue
		}
		if u, ok := obj.(*unstructured.Unstructured); ok {
			u.SetGroupVersionKind(gvk)
		}
		if typed, ok := obj.(ctrlclient.Object); ok {
			fn(gvk, typed)
		}
	}
}

func defaultRESTMapperForScheme(scheme *runtime.Scheme) *meta.DefaultRESTMapper {
	mapper := meta.NewDefaultRESTMapper(nil)
	forEachObjectKind(scheme, func(gvk schema.GroupVersionKind, _ ctrlclient.Object) {
		scope := meta.RESTScopeNamespace
		if clusterScopedKinds[gvk.GroupKind()] {
			scope = meta.RESTScopeRoot
		}
		plural, singular := meta.UnsafeGuessKindToResource(gvk)
		mapper.AddSpecific(gvk, plural, singular, scope)
	})
	return mapper
}

// clusterScopedKinds lists built-in cluster-scoped kinds. Other kinds,
// including all custom resources, are treated as namespaced.
var clusterScopedKinds = map[schema.GroupKind]bool{
	{Kind: "Namespace"}:        true,
	{Kind: "Node"}:             true,
	{Kind: "PersistentVolume"}: true,
	{Kind: "ComponentStatus"}:  true,
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}:                       true,
	{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"}:                true,
	{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}:               true,
	{Group: "apiregistration.k8s.io", Kind: "APIService"}:                           true,
	{Group: "storage.k8s.io", Kind: "StorageClass"}:                                 true,
	{Group: "storage.k8s.io", Kind: "VolumeAttachment"}:                             true,
	{Group: "storage.k8s.io", Kind: "CSIDriver"}:                                    true,
	{Group: "storage.k8s.io", Kind: "CSINode"}:                                      true,
	{Group: "scheduling.k8s.io", Kind: "PriorityClass"}:                             true,
	{Group: "node.k8s.io", Kind: "RuntimeClass"}:                                    true,
	{Group: "networking.k8s.io", Kind: "IngressClass"}:                              true,
	{Group: "certificates.k8s.io", Kind: "CertificateSigningRequest"}:               true,
	{Group: "admissionregistration.k8s.io", Kind: "ValidatingWebhookConfiguration"}: true,
	{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"}:   true,
	{Group: "flowcontrol.apiserver.k8s.io", Kind: "FlowSchema"}:                     true,
	{Group: "flowcontrol.apiserver.k8s.io", Kind: "PriorityLevelConfiguration"}:     true,
}
