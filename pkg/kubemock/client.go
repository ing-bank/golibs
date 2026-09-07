// Package kubemock provides utilities for testing Kubernetes event-based listener applications.
//
// It wraps the client-go fake client to enable testing of Kubernetes resource operations
// with realistic behavior, including DryRun support and proper error handling.
//
// Primary Use Case:
// Testing event-based listener applications that watch or react to Kubernetes resource changes.
// The mock client simulates real Kubernetes API behavior without requiring a running cluster.
//
// Features:
//
//   - DryRun Support: Properly handles DryRun flag in create, update, patch, and delete operations.
//   - Error Simulation: Returns appropriate Kubernetes API errors (Conflict, NotFound) based on operation type.
//   - In-Memory Tracking: Uses ObjectTracker to maintain state of resources during tests.
//   - Fake Client: Pre-configured fake Clientset with DryRun reactor already registered.
//
// Usage:
//
//	// Create a mock client for testing
//	client := kubemock.NewFakeClient()
//
//	// Use like a normal Kubernetes client
//	pod := &corev1.Pod{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "test-pod",
//			Namespace: "default",
//		},
//		Spec: corev1.PodSpec{
//			// ...
//		},
//	}
//
//	// Create a pod
//	created, err := client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// DryRun test - validates without persisting
//	_, err = client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{
//		DryRun: []string{metav1.DryRunAll},
//	})
//	// err will be Conflict since pod already exists
package kubemock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	testing "k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// SharedFake holds one tracker and one controller-runtime client so a manager and a
// Kubernetes-backed store can operate against the same in-memory state without creating
// different fake backends for each layer.
type SharedFake struct {
	scheme  *runtime.Scheme
	tracker testing.ObjectTracker
	client  ctrlclient.Client
	dynamic dynamic.Interface
	cache   *fakeCache
}

func NewSharedFake(scheme *runtime.Scheme, initObjs ...runtime.Object) *SharedFake {
	if scheme == nil {
		scheme = clientgoscheme.Scheme
	}
	ensureListKindsForScheme(scheme)

	tracker := newGenerationTrackingObjectTracker(scheme)
	for _, obj := range initObjs {
		if obj == nil {
			continue
		}
		if err := tracker.Add(obj); err != nil {
			panic(fmt.Sprintf("failed to add initial object to shared tracker: %v", err))
		}
	}

	builder := ctrlfake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(defaultRESTMapperForScheme(scheme)).
		WithObjectTracker(tracker)
	for gvk := range scheme.AllKnownTypes() {
		if gvk.Empty() || gvk.Kind == "" || strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		obj, err := scheme.New(gvk)
		if err != nil || obj == nil {
			continue
		}
		if typed, ok := obj.(ctrlclient.Object); ok {
			builder = builder.WithStatusSubresource(typed)
		}
	}
	client := builder.Build()
	cache := newFakeCache(client)
	shared := &SharedFake{scheme: scheme, tracker: tracker, client: client, cache: cache}
	shared.dynamic = newSharedDynamicClient(scheme, tracker, cache)
	return shared
}

func (s *SharedFake) Client() ctrlclient.Client {
	if s == nil {
		return nil
	}
	return s.client
}

func (s *SharedFake) Tracker() testing.ObjectTracker {
	if s == nil {
		return nil
	}
	return s.tracker
}

func (s *SharedFake) DynamicClient() dynamic.Interface {
	if s == nil {
		return nil
	}
	return s.dynamic
}

func (s *SharedFake) ResourceClient(gvr schema.GroupVersionResource, namespace string) dynamic.ResourceInterface {
	if s == nil || s.dynamic == nil {
		return nil
	}
	resource := s.dynamic.Resource(gvr)
	if namespace == "" {
		return resource
	}
	return resource.Namespace(namespace)
}

type sharedDynamicClient struct {
	scheme    *runtime.Scheme
	tracker   testing.ObjectTracker
	cache     *fakeCache
	listKinds map[schema.GroupVersionResource]string
}

type sharedDynamicResourceClient struct {
	client    *sharedDynamicClient
	resource  schema.GroupVersionResource
	namespace string
	listKind  string
}

func newSharedDynamicClient(scheme *runtime.Scheme, tracker testing.ObjectTracker, cache *fakeCache) dynamic.Interface {
	if scheme == nil {
		scheme = clientgoscheme.Scheme
	}
	listKinds := map[schema.GroupVersionResource]string{}
	for gvk := range scheme.AllKnownTypes() {
		if gvk.Empty() || gvk.Kind == "" || strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		resource, _ := meta.UnsafeGuessKindToResource(gvk)
		if resource.Empty() {
			continue
		}
		listKinds[resource] = gvk.Kind + "List"
	}
	return &sharedDynamicClient{scheme: scheme, tracker: tracker, cache: cache, listKinds: listKinds}
}

func (c *sharedDynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &sharedDynamicResourceClient{client: c, resource: gvr, listKind: c.listKinds[gvr]}
}

func (c *sharedDynamicClient) IsWatchListSemanticsUnSupported() bool {
	return true
}

func (c *sharedDynamicResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	clone := *c
	clone.namespace = ns
	return &clone
}

func (c *sharedDynamicResourceClient) normalizeObjectForTracker(obj *unstructured.Unstructured) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}
	if c == nil || c.client == nil || c.client.scheme == nil {
		return obj, nil
	}
	gvk := obj.GroupVersionKind()
	if gvk.Empty() {
		kind := obj.GetKind()
		if kind == "" {
			kind = c.resource.Resource
		}
		gvk = c.resource.GroupVersion().WithKind(kind)
	}
	if typedObj, err := c.client.scheme.New(gvk); err == nil {
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, typedObj); err != nil {
			if err := c.client.scheme.Convert(obj, typedObj, nil); err != nil {
				return nil, err
			}
		}
		return typedObj, nil
	}
	return obj, nil
}

func (c *sharedDynamicResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, nil
	}
	trackerObj, err := c.normalizeObjectForTracker(obj)
	if err != nil {
		return nil, err
	}
	if err := c.client.tracker.Create(c.resource, trackerObj, c.namespace, opts); err != nil {
		return nil, err
	}
	c.client.notify("add", obj, nil)
	return obj.DeepCopy(), nil
}

func (c *sharedDynamicResourceClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, nil
	}
	oldObj, err := c.client.tracker.Get(c.resource, c.namespace, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	trackerObj, err := c.normalizeObjectForTracker(obj)
	if err != nil {
		return nil, err
	}
	if err := c.client.tracker.Update(c.resource, trackerObj, c.namespace, opts); err != nil {
		return nil, err
	}
	c.client.notify("update", obj, oldObj)
	return obj.DeepCopy(), nil
}

func (c *sharedDynamicResourceClient) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return c.Update(ctx, obj, opts, "status")
}

func (c *sharedDynamicResourceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	oldObj, err := c.client.tracker.Get(c.resource, c.namespace, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err := c.client.tracker.Delete(c.resource, c.namespace, name, opts); err != nil {
		return err
	}
	c.client.notify("delete", oldObj, nil)
	return nil
}

func (c *sharedDynamicResourceClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return nil
}

func (c *sharedDynamicResourceClient) Get(ctx context.Context, name string, opts metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	obj, err := c.client.tracker.Get(c.resource, c.namespace, name, opts)
	if err != nil {
		return nil, err
	}
	return convertRuntimeObjectToUnstructured(obj, c.client.scheme)
}

func (c *sharedDynamicResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	listKind := c.listKind
	if listKind == "" {
		listKind = c.resource.Resource + "List"
	}
	listGVK := c.resource.GroupVersion().WithKind(listKind)
	baseListGVK := listGVK
	if strings.HasSuffix(baseListGVK.Kind, "List") {
		baseListGVK.Kind = baseListGVK.Kind[:len(baseListGVK.Kind)-4]
	}
	obj, err := c.client.tracker.List(c.resource, baseListGVK, c.namespace, opts)
	if err != nil {
		return nil, err
	}
	list, err := convertRuntimeObjectToUnstructuredList(obj, c.client.scheme)
	if err != nil {
		return nil, err
	}
	list.SetGroupVersionKind(listGVK)
	return list, nil
}

func (c *sharedDynamicResourceClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.tracker.Watch(c.resource, c.namespace, opts)
}

func (c *sharedDynamicResourceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	current, err := c.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if pt == types.ApplyPatchType {
		var patchObj map[string]interface{}
		if err := json.Unmarshal(data, &patchObj); err == nil {
			current.Object = mergeMaps(current.Object, patchObj)
		}
		return c.Update(ctx, current, metav1.UpdateOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager}, subresources...)
	}
	if err := json.Unmarshal(data, &current.Object); err != nil {
		return nil, err
	}
	return c.Update(ctx, current, metav1.UpdateOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager}, subresources...)
}

func (c *sharedDynamicResourceClient) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, nil
	}
	obj.SetName(name)
	trackerObj, err := c.normalizeObjectForTracker(obj)
	if err != nil {
		return nil, err
	}
	if err := c.client.tracker.Apply(c.resource, trackerObj, c.namespace, metav1.PatchOptions{
		Force:        &options.Force,
		DryRun:       options.DryRun,
		FieldManager: options.FieldManager,
	}); err != nil {
		return nil, err
	}
	result, err := c.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	c.client.notify("update", result, nil)
	return result, nil
}

func (c *sharedDynamicResourceClient) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return c.Apply(ctx, name, obj, options, "status")
}

func (c *sharedDynamicClient) notify(eventType string, obj interface{}, oldObj interface{}) {
	if c == nil || c.cache == nil || obj == nil {
		return
	}
	if c.cache.client == nil {
		return
	}
	c.cache.notify(eventType, obj, oldObj)
}

func convertRuntimeObjectToUnstructured(obj runtime.Object, scheme *runtime.Scheme) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, nil
	}
	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u.DeepCopy(), nil
	}
	if values, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj); err == nil {
		return &unstructured.Unstructured{Object: values}, nil
	}
	u := &unstructured.Unstructured{}
	if scheme != nil {
		if err := scheme.Convert(obj, u, nil); err != nil {
			return nil, err
		}
		return u, nil
	}
	return nil, fmt.Errorf("no runtime scheme available to convert object %T", obj)
}

func convertRuntimeObjectToUnstructuredList(obj runtime.Object, scheme *runtime.Scheme) (*unstructured.UnstructuredList, error) {
	if obj == nil {
		return nil, nil
	}
	if list, ok := obj.(*unstructured.UnstructuredList); ok {
		return list.DeepCopy(), nil
	}
	if values, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj); err == nil {
		out := &unstructured.UnstructuredList{}
		if fromErr := runtime.DefaultUnstructuredConverter.FromUnstructured(values, out); fromErr == nil {
			return out, nil
		}
		out.Object = values
		return out, nil
	}
	out := &unstructured.UnstructuredList{}
	if scheme != nil {
		if err := scheme.Convert(obj, out, nil); err != nil {
			return nil, err
		}
		return out, nil
	}
	return nil, fmt.Errorf("no runtime scheme available to convert list %T", obj)
}

func mergeMaps(base map[string]interface{}, patch map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}
	for key, value := range patch {
		base[key] = value
	}
	return base
}

func ensureListKindsForScheme(s *runtime.Scheme) {
	if s == nil {
		return
	}
	for gvk := range s.AllKnownTypes() {
		if gvk.Empty() || gvk.Kind == "" || strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
		if _, err := s.New(listGVK); err != nil {
			s.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})
		}
	}
}

// NewFakeClient returns a Kubernetes fake client with DryRun support
func NewFakeClient() *fake.Clientset {
	client := fake.NewSimpleClientset()
	tracker, _ := client.Tracker().(testing.ObjectTracker)
	client.Fake.PrependReactor("*", "*", generationReactor(tracker))
	client.Fake.PrependReactor("*", "*", DryRunReactor(tracker))
	return client
}

func generationReactor(tracker testing.ObjectTracker) func(action testing.Action) (handled bool, ret runtime.Object, err error) {
	return func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case testing.CreateActionImpl:
			bumpGeneration(a.GetObject(), true)
		case testing.UpdateActionImpl:
			populateGenerationFromExisting(tracker, a.GetResource(), a.GetNamespace(), a.GetObject())
			bumpGeneration(a.GetObject(), false)
		case testing.PatchActionImpl:
			if a.GetPatchType() == types.ApplyPatchType || a.GetPatchType() == types.MergePatchType || a.GetPatchType() == types.StrategicMergePatchType || a.GetPatchType() == types.JSONPatchType {
				bumpGenerationForPatch(tracker, a.GetResource(), a.GetNamespace(), a.GetName())
			}
		}
		return false, nil, nil
	}
}

func populateGenerationFromExisting(tracker testing.ObjectTracker, gvr schema.GroupVersionResource, ns string, obj runtime.Object) {
	if tracker == nil || obj == nil {
		return
	}
	accessor, err := meta.Accessor(obj)
	if err != nil || accessor == nil || accessor.GetName() == "" || accessor.GetGeneration() != 0 {
		return
	}
	current, err := tracker.Get(gvr, ns, accessor.GetName(), metav1.GetOptions{})
	if err != nil {
		return
	}
	currentAccessor, err := meta.Accessor(current)
	if err != nil || currentAccessor == nil {
		return
	}
	accessor.SetGeneration(currentAccessor.GetGeneration())
}

// NewFakeControllerRuntimeClient creates a controller-runtime client backed by the same fake
// object tracker used by client-go's fake clientset. This lets tests build a manager using the
// standard controller-runtime client.Client contract without reimplementing CRUD logic by hand.
func NewFakeControllerRuntimeClient(initObjs ...runtime.Object) ctrlclient.Client {
	return NewControllerRuntimeClientFromFakeClient(NewFakeClient(), nil, nil, initObjs...)
}

// NewControllerRuntimeClientFromFakeClient creates a controller-runtime client using a client-go
// fake clientset as the backing object tracker.
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

	builder := ctrlfake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(restMapper).
		WithObjectTracker(&generationTrackingObjectTracker{ObjectTracker: cs.Tracker()})

	if len(initObjs) > 0 {
		builder = builder.WithRuntimeObjects(initObjs...)
	}
	return builder.Build()
}

func defaultRESTMapperForScheme(s *runtime.Scheme) meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	for gvk := range s.AllKnownTypes() {
		if gvk.Empty() || gvk.Kind == "" {
			continue
		}
		resource, _ := meta.UnsafeGuessKindToResource(gvk)
		scope := meta.RESTScopeNamespace
		if isClusterScopedKind(gvk) {
			scope = meta.RESTScopeRoot
		}
		mapper.AddSpecific(gvk, resource, resource, scope)
	}
	return mapper
}

func isClusterScopedKind(gvk schema.GroupVersionKind) bool {
	switch gvk.Kind {
	case "Namespace", "Node", "PersistentVolume", "ClusterRole", "ClusterRoleBinding",
		"CustomResourceDefinition", "StorageClass", "PriorityClass", "RuntimeClass",
		"VolumeAttachment", "ValidatingWebhookConfiguration", "MutatingWebhookConfiguration",
		"APIService", "PodSecurityPolicy", "IngressClass", "CSIDriver", "CSINode",
		"CertificateSigningRequest", "PriorityLevelConfiguration", "FlowSchema":
		return true
	default:
		return false
	}
}

func DryRunReactor(tracker testing.ObjectTracker) func(action testing.Action) (handled bool, ret runtime.Object, err error) {
	return func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case testing.CreateActionImpl:
			if opts := a.GetCreateOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				obj := a.GetObject()
				accessor, err := meta.Accessor(obj)
				if err != nil {
					return true, nil, err
				}
				ns := a.GetNamespace()
				name := accessor.GetName()
				_, err = tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err == nil {
					return true, nil, errors.NewConflict(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name, nil)
				}
				return true, a.GetObject(), nil
			}
		case testing.UpdateActionImpl:
			if opts := a.GetUpdateOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				obj := a.GetObject()
				accessor, err := meta.Accessor(obj)
				if err != nil {
					return true, nil, err
				}
				ns := a.GetNamespace()
				name := accessor.GetName()
				_, err = tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, a.GetObject(), nil
			}

		case testing.PatchActionImpl:
			if opts := a.GetPatchOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				ns := a.GetNamespace()
				name := a.GetName()
				_, err := tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, nil, nil
			}

		case testing.DeleteActionImpl:
			if opts := a.GetDeleteOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				ns := a.GetNamespace()
				name := a.GetName()
				_, err := tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, nil, nil
			}
		}
		return false, nil, nil
	}
}

type generationTrackingObjectTracker struct {
	testing.ObjectTracker
}

func newGenerationTrackingObjectTracker(scheme *runtime.Scheme) testing.ObjectTracker {
	if scheme == nil {
		scheme = clientgoscheme.Scheme
	}
	return &generationTrackingObjectTracker{
		ObjectTracker: testing.NewObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder()),
	}
}

func (t *generationTrackingObjectTracker) Add(obj runtime.Object) error {
	bumpGeneration(obj, true)
	return t.ObjectTracker.Add(obj)
}

func (t *generationTrackingObjectTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.CreateOptions) error {
	bumpGeneration(obj, true)
	return t.ObjectTracker.Create(gvr, obj, ns, opts...)
}

func (t *generationTrackingObjectTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.UpdateOptions) error {
	if accessor, err := meta.Accessor(obj); err == nil && accessor != nil {
		if current, currentErr := t.ObjectTracker.Get(gvr, ns, accessor.GetName(), metav1.GetOptions{}); currentErr == nil {
			if currentAccessor, currentAccessorErr := meta.Accessor(current); currentAccessorErr == nil && currentAccessor != nil {
				if accessor.GetGeneration() == 0 {
					accessor.SetGeneration(currentAccessor.GetGeneration())
				}
			}
		}
	}
	bumpGeneration(obj, false)
	return t.ObjectTracker.Update(gvr, obj, ns, opts...)
}

func (t *generationTrackingObjectTracker) Patch(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	if accessor, err := meta.Accessor(obj); err == nil && accessor != nil {
		if current, currentErr := t.ObjectTracker.Get(gvr, ns, accessor.GetName(), metav1.GetOptions{}); currentErr == nil {
			if currentAccessor, currentAccessorErr := meta.Accessor(current); currentAccessorErr == nil && currentAccessor != nil {
				if accessor.GetGeneration() == 0 {
					accessor.SetGeneration(currentAccessor.GetGeneration())
				}
			}
		}
	}
	bumpGeneration(obj, false)
	return t.ObjectTracker.Patch(gvr, obj, ns, opts...)
}

func (t *generationTrackingObjectTracker) Apply(gvr schema.GroupVersionResource, applyConfiguration runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	if accessor, err := meta.Accessor(applyConfiguration); err == nil && accessor != nil {
		if current, currentErr := t.ObjectTracker.Get(gvr, ns, accessor.GetName(), metav1.GetOptions{}); currentErr == nil {
			if currentAccessor, currentAccessorErr := meta.Accessor(current); currentAccessorErr == nil && currentAccessor != nil {
				if accessor.GetGeneration() == 0 {
					accessor.SetGeneration(currentAccessor.GetGeneration())
				}
			}
		}
	}
	bumpGeneration(applyConfiguration, false)
	return t.ObjectTracker.Apply(gvr, applyConfiguration, ns, opts...)
}

func bumpGeneration(obj runtime.Object, isCreate bool) {
	if obj == nil {
		return
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	generation := accessor.GetGeneration()
	switch {
	case isCreate && generation == 0:
		accessor.SetGeneration(1)
	case !isCreate:
		if generation == 0 {
			accessor.SetGeneration(1)
			return
		}
		accessor.SetGeneration(generation + 1)
	}
}

func bumpGenerationForPatch(tracker testing.ObjectTracker, gvr schema.GroupVersionResource, ns, name string) {
	if tracker == nil || gvr.Empty() || name == "" {
		return
	}
	current, err := tracker.Get(gvr, ns, name, metav1.GetOptions{})
	if err != nil {
		return
	}
	bumpGeneration(current, false)
}
