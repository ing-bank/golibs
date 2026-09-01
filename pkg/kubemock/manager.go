package kubemock

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// NewManager returns a controller-runtime manager wired to the kubemock fake client.
//
// It is intended for unit tests and local integration tests where you want to construct
// a manager and pass mgr.GetClient() into reconcilers without a real cluster.
//
// The fake manager disables leader election and binds metrics/health endpoints to port 0,
// and it installs a lightweight in-memory cache so the manager can register informers and
// observe fake add/update/delete events without a real API server.
func NewManager(cfg *rest.Config, opts ctrl.Options, initObjs ...runtime.Object) (ctrl.Manager, error) {
	if cfg == nil {
		cfg = &rest.Config{Host: "https://fake.invalid"}
	}
	if opts.Scheme == nil {
		opts.Scheme = clientgoscheme.Scheme
	}
	_ = corev1.AddToScheme(opts.Scheme)
	fakeClient := NewControllerRuntimeClientFromFakeClient(NewFakeClient(), opts.Scheme, defaultRESTMapperForScheme(opts.Scheme), initObjs...)
	return NewManagerWithClient(cfg, opts, fakeClient)
}

// NewManagerWithClient builds a controller-runtime manager around an injected fake client.
func NewManagerWithClient(cfg *rest.Config, opts ctrl.Options, fakeClient client.Client) (ctrl.Manager, error) {
	if cfg == nil {
		cfg = &rest.Config{Host: "https://fake.invalid"}
	}
	if opts.Scheme == nil {
		opts.Scheme = clientgoscheme.Scheme
	}
	_ = corev1.AddToScheme(opts.Scheme)
	if fakeClient == nil {
		fakeClient = NewControllerRuntimeClientFromFakeClient(NewFakeClient(), opts.Scheme, defaultRESTMapperForScheme(opts.Scheme))
	}

	cacheClient := newFakeCache(fakeClient)
	eventClient := &fakeEventingClient{Client: fakeClient, cache: cacheClient}

	opts.NewClient = func(_ *rest.Config, _ client.Options) (client.Client, error) {
		return eventClient, nil
	}
	opts.NewCache = func(_ *rest.Config, _ ctrlcache.Options) (ctrlcache.Cache, error) {
		return cacheClient, nil
	}
	opts.LeaderElection = false
	opts.Metrics = metricsserver.Options{BindAddress: "0"}
	opts.HealthProbeBindAddress = "0"
	opts.PprofBindAddress = "0"

	return ctrl.NewManager(cfg, opts)
}

// NewManagerWithSharedFake builds a controller-runtime manager that reuses the same in-memory
// object tracker and cache as a shared fake instance.
func NewManagerWithSharedFake(cfg *rest.Config, opts ctrl.Options, shared *SharedFake) (ctrl.Manager, error) {
	if cfg == nil {
		cfg = &rest.Config{Host: "https://fake.invalid"}
	}
	if shared == nil {
		shared = NewSharedFake(opts.Scheme)
	}
	if opts.Scheme == nil {
		opts.Scheme = shared.scheme
	}
	_ = corev1.AddToScheme(opts.Scheme)
	if shared.cache == nil {
		shared.cache = newFakeCache(shared.client)
	}
	if shared.client == nil {
		shared.client = NewControllerRuntimeClientFromFakeClient(NewFakeClient(), opts.Scheme, defaultRESTMapperForScheme(opts.Scheme))
		shared.cache = newFakeCache(shared.client)
	}

	eventClient := &fakeEventingClient{Client: shared.client, cache: shared.cache}
	opts.NewClient = func(_ *rest.Config, _ client.Options) (client.Client, error) {
		return eventClient, nil
	}
	opts.NewCache = func(_ *rest.Config, _ ctrlcache.Options) (ctrlcache.Cache, error) {
		return shared.cache, nil
	}
	opts.LeaderElection = false
	opts.Metrics = metricsserver.Options{BindAddress: "0"}
	opts.HealthProbeBindAddress = "0"
	opts.PprofBindAddress = "0"

	return ctrl.NewManager(cfg, opts)
}

type fakeCache struct {
	mu        sync.Mutex
	client    client.Client
	informers map[schema.GroupVersionKind]*fakeInformer
}

func newFakeCache(client client.Client) *fakeCache {
	return &fakeCache{
		client:    client,
		informers: map[schema.GroupVersionKind]*fakeInformer{},
	}
}

func (c *fakeCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.client == nil {
		return nil
	}
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *fakeCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.client == nil {
		return nil
	}
	return c.client.List(ctx, list, opts...)
}

func (c *fakeCache) GetInformer(ctx context.Context, obj client.Object, opts ...ctrlcache.InformerGetOption) (ctrlcache.Informer, error) {
	gvk, err := objectGVK(c.client, obj)
	if err != nil {
		return nil, err
	}
	return c.getInformer(gvk), nil
}

func (c *fakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...ctrlcache.InformerGetOption) (ctrlcache.Informer, error) {
	return c.getInformer(gvk), nil
}

func (c *fakeCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	gvk, err := objectGVK(c.client, obj)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.informers, gvk)
	return nil
}

func (c *fakeCache) Start(ctx context.Context) error {
	return nil
}

func (c *fakeCache) WaitForCacheSync(ctx context.Context) bool {
	return true
}

func (c *fakeCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return nil
}

func (c *fakeCache) getInformer(gvk schema.GroupVersionKind) *fakeInformer {
	c.mu.Lock()
	defer c.mu.Unlock()
	if informer, ok := c.informers[gvk]; ok {
		return informer
	}
	informer := &fakeInformer{gvk: gvk, objects: map[string]interface{}{}}
	c.informers[gvk] = informer
	return informer
}

func objectGVK(c client.Client, obj client.Object) (schema.GroupVersionKind, error) {
	if obj == nil {
		return schema.GroupVersionKind{}, fmt.Errorf("nil object")
	}
	gvks, _, err := c.Scheme().ObjectKinds(obj)
	if err != nil || len(gvks) == 0 {
		if gvk := obj.GetObjectKind().GroupVersionKind(); !gvk.Empty() {
			return gvk, nil
		}
		return schema.GroupVersionKind{}, fmt.Errorf("could not determine GVK for object %T: %w", obj, err)
	}
	return gvks[0], nil
}

type fakeInformer struct {
	mu       sync.Mutex
	gvk      schema.GroupVersionKind
	objects  map[string]interface{}
	handlers []toolscache.ResourceEventHandler
}

func (i *fakeInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(handler)
}

func (i *fakeInformer) AddEventHandlerWithResyncPeriod(handler toolscache.ResourceEventHandler, resyncPeriod time.Duration) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(handler)
}

func (i *fakeInformer) AddEventHandlerWithOptions(handler toolscache.ResourceEventHandler, options toolscache.HandlerOptions) (toolscache.ResourceEventHandlerRegistration, error) {
	return i.addHandler(handler)
}

func (i *fakeInformer) RemoveEventHandler(handle toolscache.ResourceEventHandlerRegistration) error {
	return nil
}

func (i *fakeInformer) AddIndexers(indexers toolscache.Indexers) error {
	return nil
}

func (i *fakeInformer) HasSynced() bool {
	return true
}

func (i *fakeInformer) HasSyncedChecker() toolscache.DoneChecker {
	return fakeDoneChecker{}
}

func (i *fakeInformer) IsStopped() bool {
	return false
}

func (i *fakeInformer) addHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	if handler == nil {
		return nil, fmt.Errorf("nil event handler")
	}
	i.mu.Lock()
	i.handlers = append(i.handlers, handler)
	objects := make([]interface{}, 0, len(i.objects))
	for _, obj := range i.objects {
		objects = append(objects, obj)
	}
	i.mu.Unlock()
	for _, obj := range objects {
		handler.OnAdd(obj, false)
	}
	return &fakeResourceEventHandlerRegistration{informer: i, handler: handler}, nil
}

func (i *fakeInformer) emit(eventType string, obj interface{}, oldObj interface{}) {
	i.mu.Lock()
	handlers := append([]toolscache.ResourceEventHandler(nil), i.handlers...)
	i.mu.Unlock()
	for _, handler := range handlers {
		switch eventType {
		case "add":
			handler.OnAdd(obj, false)
		case "update":
			handler.OnUpdate(oldObj, obj)
		case "delete":
			handler.OnDelete(obj)
		}
	}
}

func (i *fakeInformer) storeObject(obj interface{}) {
	if obj == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	key, _ := toolscache.MetaNamespaceKeyFunc(obj)
	if key == "" {
		key = fmt.Sprintf("%T/%p", obj, obj)
	}
	i.objects[key] = obj
}

func (i *fakeInformer) deleteObject(obj interface{}) {
	if obj == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	key, _ := toolscache.MetaNamespaceKeyFunc(obj)
	if key == "" {
		key = fmt.Sprintf("%T/%p", obj, obj)
	}
	delete(i.objects, key)
}

type fakeResourceEventHandlerRegistration struct {
	informer *fakeInformer
	handler  toolscache.ResourceEventHandler
}

func (f *fakeResourceEventHandlerRegistration) HasSynced() bool {
	return true
}

func (f *fakeResourceEventHandlerRegistration) HasSyncedChecker() toolscache.DoneChecker {
	return fakeDoneChecker{}
}

type fakeDoneChecker struct{}

type fakeEventingClient struct {
	client.Client
	cache *fakeCache
}

func (c *fakeEventingClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if err := c.Client.Create(ctx, obj, opts...); err != nil {
		return err
	}
	c.notify("add", obj, nil)
	return nil
}

func (c *fakeEventingClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	before := obj.DeepCopyObject()
	if err := c.Client.Update(ctx, obj, opts...); err != nil {
		return err
	}
	c.notify("update", obj, before)
	return nil
}

func (c *fakeEventingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	before := obj.DeepCopyObject()
	if err := c.Client.Delete(ctx, obj, opts...); err != nil {
		return err
	}
	c.notify("delete", before, nil)
	return nil
}

func (c *fakeEventingClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	before := obj.DeepCopyObject()
	if err := c.Client.Patch(ctx, obj, patch, opts...); err != nil {
		return err
	}
	c.notify("update", obj, before)
	return nil
}

func (c *fakeEventingClient) notify(eventType string, obj interface{}, oldObj interface{}) {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.notify(eventType, obj, oldObj)
}

func (f fakeDoneChecker) Name() string { return "kubemock-fake-informer" }
func (f fakeDoneChecker) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (c *fakeCache) notify(eventType string, obj interface{}, oldObj interface{}) {
	if c == nil || obj == nil {
		return
	}
	var gvk schema.GroupVersionKind
	if typed, ok := obj.(client.Object); ok {
		var err error
		gvk, err = objectGVK(c.client, typed)
		if err != nil {
			if runtimeObj, ok := obj.(runtime.Object); ok {
				gvk = runtimeObj.GetObjectKind().GroupVersionKind()
			}
		}
	} else if runtimeObj, ok := obj.(runtime.Object); ok {
		gvk = runtimeObj.GetObjectKind().GroupVersionKind()
	}
	if gvk.Empty() {
		return
	}
	informer := c.getInformer(gvk)
	switch eventType {
	case "add":
		informer.storeObject(obj)
		informer.emit("add", obj, nil)
	case "update":
		informer.storeObject(obj)
		informer.emit("update", obj, oldObj)
	case "delete":
		informer.deleteObject(obj)
		informer.emit("delete", obj, nil)
	}
}
