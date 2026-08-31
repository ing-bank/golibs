package kubemock

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// NewManager returns a controller-runtime manager wired to the kubemock fake client.
//
// It is intended for unit tests and local integration tests where you want to construct
// a manager and pass mgr.GetClient() into reconcilers without a real cluster.
//
// The fake manager disables leader election and binds metrics/health endpoints to port 0,
// and it installs a no-op cache so the manager can be created and used without any API
// server or informer startup.
func NewManager(cfg *rest.Config, opts ctrl.Options, initObjs ...runtime.Object) (ctrl.Manager, error) {
	if cfg == nil {
		cfg = &rest.Config{Host: "https://fake.invalid"}
	}
	if opts.Scheme == nil {
		opts.Scheme = clientgoscheme.Scheme
	}

	// Ensure the common built-ins are registered in the scheme by default so the fake manager can
	// resolve namespaced resource metadata just like a normal controller-runtime client.
	_ = corev1.AddToScheme(opts.Scheme)

	fakeClient := NewFakeControllerRuntimeClient(initObjs...)
	cacheClient := &fakeCache{client: fakeClient}

	opts.NewClient = func(_ *rest.Config, _ client.Options) (client.Client, error) {
		return fakeClient, nil
	}
	opts.NewCache = func(_ *rest.Config, _ cache.Options) (cache.Cache, error) {
		return cacheClient, nil
	}
	opts.LeaderElection = false
	opts.Metrics = metricsserver.Options{BindAddress: "0"}
	opts.HealthProbeBindAddress = "0"
	opts.PprofBindAddress = "0"

	return ctrl.NewManager(cfg, opts)
}

type fakeCache struct {
	client client.Client
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

func (c *fakeCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return &fakeInformer{}, nil
}

func (c *fakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return &fakeInformer{}, nil
}

func (c *fakeCache) RemoveInformer(ctx context.Context, obj client.Object) error {
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

type fakeInformer struct{}

func (i *fakeInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (i *fakeInformer) AddEventHandlerWithResyncPeriod(handler toolscache.ResourceEventHandler, resyncPeriod time.Duration) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

func (i *fakeInformer) AddEventHandlerWithOptions(handler toolscache.ResourceEventHandler, options toolscache.HandlerOptions) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, nil
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

type fakeDoneChecker struct{}

func (f fakeDoneChecker) Name() string { return "kubemock-fake-informer" }
func (f fakeDoneChecker) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
