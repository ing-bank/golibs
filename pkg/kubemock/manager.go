package kubemock

import (
	"context"
	"sort"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// NewManager returns a controller-runtime manager backed by a new FakeCluster
// for opts.Scheme (client-go's scheme when nil) seeded with initObjs.
//
// The manager disables leader election, binds metrics, health and pprof
// endpoints to port 0, and uses an in-memory cache whose informers receive
// add/update/delete events for writes made through mgr.GetClient().
func NewManager(cfg *rest.Config, opts ctrl.Options, initObjs ...runtime.Object) (ctrl.Manager, error) {
	if opts.Scheme == nil {
		opts.Scheme = clientgoscheme.Scheme
	}
	return NewManagerForFakeCluster(cfg, opts, NewFakeCluster(opts.Scheme, initObjs...))
}

// NewManagerWithClient builds a fake manager around an existing fake client.
// Writes through mgr.GetClient() emit informer events; writes made directly on
// fakeClient do not.
func NewManagerWithClient(cfg *rest.Config, opts ctrl.Options, fakeClient client.Client) (ctrl.Manager, error) {
	if fakeClient == nil {
		return NewManager(cfg, opts)
	}
	if opts.Scheme == nil {
		opts.Scheme = fakeClient.Scheme()
	}
	cache := newFakeCache(fakeClient)
	return newFakeManager(cfg, opts, &eventingClient{Client: fakeClient, cache: cache}, cache)
}

// NewManagerForFakeCluster builds a fake manager on top of cluster, so writes
// through cluster.Client(), cluster.DynamicClient() and mgr.GetClient() all
// reach the manager's informers.
func NewManagerForFakeCluster(cfg *rest.Config, opts ctrl.Options, cluster *FakeCluster) (ctrl.Manager, error) {
	if cluster == nil {
		cluster = NewFakeCluster(opts.Scheme)
	}
	if opts.Scheme == nil {
		opts.Scheme = cluster.scheme
	}
	return newFakeManager(cfg, opts, cluster.client, cluster.cache)
}

func newFakeManager(cfg *rest.Config, opts ctrl.Options, c client.Client, cache ctrlcache.Cache) (ctrl.Manager, error) {
	if cfg == nil {
		cfg = &rest.Config{Host: "https://fake.invalid"}
	}
	opts.NewClient = func(*rest.Config, client.Options) (client.Client, error) { return c, nil }
	opts.NewCache = func(*rest.Config, ctrlcache.Options) (ctrlcache.Cache, error) { return cache, nil }
	opts.LeaderElection = false
	opts.Metrics = metricsserver.Options{BindAddress: "0"}
	opts.HealthProbeBindAddress = "0"
	opts.PprofBindAddress = "0"
	return ctrl.NewManager(cfg, opts)
}

// fakeCache serves reads straight from the client (always consistent, unlike
// a real cache) and keeps one fakeInformer per kind.
//
// Events are derived from stored state rather than from the write call: after
// every write the object is re-read and diffed against the informer's copy.
// That makes finalizer deletes, DeleteAllOf, no-op writes and dynamic writes
// produce the same events a real watch would, from a single code path.
type fakeCache struct {
	client    client.Client
	mu        sync.Mutex
	informers map[schema.GroupVersionKind]*fakeInformer
}

func newFakeCache(c client.Client) *fakeCache {
	return &fakeCache{client: c, informers: map[schema.GroupVersionKind]*fakeInformer{}}
}

func (c *fakeCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *fakeCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.client.List(ctx, list, opts...)
}

func (c *fakeCache) GetInformer(ctx context.Context, obj client.Object, _ ...ctrlcache.InformerGetOption) (ctrlcache.Informer, error) {
	gvk, err := apiutil.GVKForObject(obj, c.client.Scheme())
	if err != nil {
		return nil, err
	}
	return c.informer(ctx, gvk), nil
}

func (c *fakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, _ ...ctrlcache.InformerGetOption) (ctrlcache.Informer, error) {
	return c.informer(ctx, gvk), nil
}

func (c *fakeCache) RemoveInformer(_ context.Context, obj client.Object) error {
	gvk, err := apiutil.GVKForObject(obj, c.client.Scheme())
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.informers, gvk)
	return nil
}

// Start returns immediately: informers are fed synchronously by writes.
func (c *fakeCache) Start(context.Context) error { return nil }

// WaitForCacheSync is always true: reads go straight to the client.
func (c *fakeCache) WaitForCacheSync(context.Context) bool { return true }

// IndexField is a no-op; field selectors need indexes registered on the
// underlying fake client builder.
func (c *fakeCache) IndexField(context.Context, client.Object, string, client.IndexerFunc) error {
	return nil
}

// informer returns the informer for gvk, creating it with the currently
// stored objects so new handlers see them, like an initial list.
func (c *fakeCache) informer(ctx context.Context, gvk schema.GroupVersionKind) *fakeInformer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if inf, ok := c.informers[gvk]; ok {
		return inf
	}
	inf := &fakeInformer{objects: map[types.NamespacedName]client.Object{}}
	for _, obj := range c.listStored(ctx, gvk) {
		inf.objects[client.ObjectKeyFromObject(obj)] = obj
	}
	c.informers[gvk] = inf
	return inf
}

// listStored returns stored objects of gvk; kinds without a registered List
// type yield nothing.
func (c *fakeCache) listStored(ctx context.Context, gvk schema.GroupVersionKind) []client.Object {
	listGVK := gvk.GroupVersion().WithKind(gvk.Kind + "List")
	listObj, err := c.client.Scheme().New(listGVK)
	if err != nil {
		return nil
	}
	if u, ok := listObj.(*unstructured.UnstructuredList); ok {
		u.SetGroupVersionKind(listGVK) // installed resources are unstructured and carry no kind
	}
	list, ok := listObj.(client.ObjectList)
	if !ok || c.client.List(ctx, list) != nil {
		return nil
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil
	}
	out := make([]client.Object, 0, len(items))
	for _, item := range items {
		if obj, ok := item.(client.Object); ok {
			out = append(out, obj)
		}
	}
	return out
}

// sync re-reads key and emits the event that brings the gvk informer in line
// with stored state.
func (c *fakeCache) sync(ctx context.Context, gvk schema.GroupVersionKind, key types.NamespacedName) {
	inf := c.informer(ctx, gvk)
	current := c.newObject(gvk)
	switch err := c.client.Get(ctx, key, current); {
	case apierrors.IsNotFound(err):
		inf.apply(key, nil)
	case err == nil:
		inf.apply(key, current)
	}
}

// syncObject syncs the stored state of obj after a write.
func (c *fakeCache) syncObject(ctx context.Context, obj runtime.Object) {
	gvk, err := apiutil.GVKForObject(obj, c.client.Scheme())
	if err != nil {
		return
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return
	}
	c.sync(ctx, gvk, types.NamespacedName{Namespace: accessor.GetNamespace(), Name: accessor.GetName()})
}

// syncKnown re-syncs every object the gvk informer knows, catching deletes
// from bulk operations.
func (c *fakeCache) syncKnown(ctx context.Context, gvk schema.GroupVersionKind) {
	for _, key := range c.informer(ctx, gvk).keys() {
		c.sync(ctx, gvk, key)
	}
}

// newObject returns an empty typed object for gvk, or unstructured if the
// scheme does not know the kind or it was installed without a Go type. Typed
// objects match what real informers deliver.
func (c *fakeCache) newObject(gvk schema.GroupVersionKind) client.Object {
	if obj, err := c.client.Scheme().New(gvk); err == nil {
		if _, isUnstructured := obj.(*unstructured.Unstructured); !isUnstructured {
			if typed, ok := obj.(client.Object); ok {
				return typed
			}
		}
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	return u
}

// fakeInformer holds the last known state per object and delivers events
// synchronously to its handlers.
type fakeInformer struct {
	mu       sync.Mutex
	objects  map[types.NamespacedName]client.Object
	handlers []*handlerRegistration
}

type handlerRegistration struct {
	handler toolscache.ResourceEventHandler
}

func (r *handlerRegistration) HasSynced() bool                          { return true }
func (r *handlerRegistration) HasSyncedChecker() toolscache.DoneChecker { return syncedChecker{} }

type syncedChecker struct{}

func (syncedChecker) Name() string { return "kubemock-fake-informer" }
func (syncedChecker) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (i *fakeInformer) AddEventHandler(h toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(h), nil
}

func (i *fakeInformer) AddEventHandlerWithResyncPeriod(h toolscache.ResourceEventHandler, _ time.Duration) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(h), nil
}

func (i *fakeInformer) AddEventHandlerWithOptions(h toolscache.ResourceEventHandler, _ toolscache.HandlerOptions) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(h), nil
}

func (i *fakeInformer) RemoveEventHandler(handle toolscache.ResourceEventHandlerRegistration) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	for idx, reg := range i.handlers {
		if reg == handle {
			i.handlers = append(i.handlers[:idx], i.handlers[idx+1:]...)
			break
		}
	}
	return nil
}

func (i *fakeInformer) AddIndexers(toolscache.Indexers) error    { return nil }
func (i *fakeInformer) HasSynced() bool                          { return true }
func (i *fakeInformer) HasSyncedChecker() toolscache.DoneChecker { return syncedChecker{} }
func (i *fakeInformer) IsStopped() bool                          { return false }

// addHandler registers h and replays known objects as adds, sorted for
// deterministic tests.
func (i *fakeInformer) addHandler(h toolscache.ResourceEventHandler) *handlerRegistration {
	reg := &handlerRegistration{handler: h}
	i.mu.Lock()
	i.handlers = append(i.handlers, reg)
	keys := i.sortedKeysLocked()
	existing := make([]client.Object, 0, len(keys))
	for _, key := range keys {
		existing = append(existing, i.objects[key])
	}
	i.mu.Unlock()
	for _, obj := range existing {
		h.OnAdd(obj, true)
	}
	return reg
}

func (i *fakeInformer) keys() []types.NamespacedName {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.sortedKeysLocked()
}

func (i *fakeInformer) sortedKeysLocked() []types.NamespacedName {
	keys := make([]types.NamespacedName, 0, len(i.objects))
	for key := range i.objects {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a].String() < keys[b].String() })
	return keys
}

// apply records current (nil when deleted) for key and emits the matching
// event. An unchanged resourceVersion means nothing was persisted: no event.
// Handlers run outside the lock so they may write back to the client.
func (i *fakeInformer) apply(key types.NamespacedName, current client.Object) {
	i.mu.Lock()
	old, known := i.objects[key]
	var notify func(toolscache.ResourceEventHandler)
	switch {
	case current == nil && known:
		delete(i.objects, key)
		notify = func(h toolscache.ResourceEventHandler) { h.OnDelete(old) }
	case current == nil:
	case !known:
		i.objects[key] = current
		notify = func(h toolscache.ResourceEventHandler) { h.OnAdd(current, false) }
	case old.GetResourceVersion() != current.GetResourceVersion():
		i.objects[key] = current
		notify = func(h toolscache.ResourceEventHandler) { h.OnUpdate(old, current) }
	}
	handlers := append([]*handlerRegistration(nil), i.handlers...)
	i.mu.Unlock()
	if notify == nil {
		return
	}
	for _, reg := range handlers {
		notify(reg.handler)
	}
}

// eventingClient syncs the cache after each successful write so informers
// observe it.
type eventingClient struct {
	client.Client
	cache *fakeCache
}

func (c *eventingClient) synced(ctx context.Context, obj runtime.Object, err error) error {
	if err == nil {
		c.cache.syncObject(ctx, obj)
	}
	return err
}

func (c *eventingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.synced(ctx, obj, c.Client.Create(ctx, obj, opts...))
}

func (c *eventingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.synced(ctx, obj, c.Client.Update(ctx, obj, opts...))
}

func (c *eventingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.synced(ctx, obj, c.Client.Patch(ctx, obj, patch, opts...))
}

func (c *eventingClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.ApplyOption) error {
	return c.syncedApply(ctx, obj, c.Client.Apply(ctx, obj, opts...))
}

func (c *eventingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.synced(ctx, obj, c.Client.Delete(ctx, obj, opts...))
}

func (c *eventingClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if err := c.Client.DeleteAllOf(ctx, obj, opts...); err != nil {
		return err
	}
	if gvk, err := apiutil.GVKForObject(obj, c.Scheme()); err == nil {
		c.cache.syncKnown(ctx, gvk)
	}
	return nil
}

// syncedApply syncs the object named by an apply configuration.
func (c *eventingClient) syncedApply(ctx context.Context, obj runtime.ApplyConfiguration, err error) error {
	if err != nil {
		return err
	}
	if values, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(obj); convErr == nil {
		c.cache.syncObject(ctx, &unstructured.Unstructured{Object: values})
	}
	return nil
}

func (c *eventingClient) Status() client.SubResourceWriter {
	return c.SubResource("status")
}

func (c *eventingClient) SubResource(subResource string) client.SubResourceClient {
	return &eventingSubResourceClient{SubResourceClient: c.Client.SubResource(subResource), parent: c}
}

// eventingSubResourceClient syncs the parent object after subresource writes.
type eventingSubResourceClient struct {
	client.SubResourceClient
	parent *eventingClient
}

func (c *eventingSubResourceClient) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return c.parent.synced(ctx, obj, c.SubResourceClient.Create(ctx, obj, subResource, opts...))
}

func (c *eventingSubResourceClient) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return c.parent.synced(ctx, obj, c.SubResourceClient.Update(ctx, obj, opts...))
}

func (c *eventingSubResourceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return c.parent.synced(ctx, obj, c.SubResourceClient.Patch(ctx, obj, patch, opts...))
}

func (c *eventingSubResourceClient) Apply(ctx context.Context, obj runtime.ApplyConfiguration, opts ...client.SubResourceApplyOption) error {
	return c.parent.syncedApply(ctx, obj, c.SubResourceClient.Apply(ctx, obj, opts...))
}
