package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

// CachedDynamicResource wraps DynamicResource with an informer-based cache
// This reduces API calls by maintaining a local cache that's updated via watch
type CachedDynamicResource[V GenericType] struct {
	*DynamicResource[V]
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
}

type CachedConfig struct {
	Config
	ResyncPeriod       metav1.Duration `json:"resyncPeriod" yaml:"resyncPeriod"`
	WaitForCacheToSync bool            `json:"waitForCacheToSync" yaml:"waitForCacheToSync"`
}

func applyDefaults(_ *CachedConfig) {}

// newCachedStore is the internal constructor that creates a cached store from a dynamic client
func newCachedStore[V GenericType](cfg CachedConfig, dynClient dynamic.Interface) (*CachedDynamicResource[V], error) {
	applyDefaults(&cfg)

	gvr := schema.GroupVersionResource{
		Group:    cfg.Group,
		Version:  cfg.Version,
		Resource: cfg.Resource,
	}

	// Create the base DynamicResource
	baseStore, err := New[V](cfg.Config, dynClient.Resource(gvr).Namespace(cfg.Namespace))
	if err != nil {
		return nil, err
	}

	// Create a dynamic informer factory
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynClient,
		cfg.ResyncPeriod.Duration,
		cfg.Namespace,
		nil, // No label selector filter
	)

	// Get the informer for this specific resource
	informer := factory.ForResource(gvr).Informer()

	cached := &CachedDynamicResource[V]{
		DynamicResource: baseStore,
		informer:        informer,
		stopCh:          make(chan struct{}, 1),
	}

	go informer.Run(cached.stopCh)

	// Wait for cache to sync
	if cfg.WaitForCacheToSync {
		if !cache.WaitForCacheSync(cached.stopCh, informer.HasSynced) {
			return nil, fmt.Errorf("timed out waiting for cache to sync")
		}
	}

	return cached, nil
}

func (c *CachedDynamicResource[V]) Run(ctx context.Context) error {
	c.informer.RunWithContext(ctx)
	return nil
}

// NewCached creates a new cached Kubernetes store from an existing dynamic client
func NewCached[V GenericType](cfg CachedConfig, client dynamic.Interface) (*CachedDynamicResource[V], error) {
	return newCachedStore[V](cfg, client)
}

// NewCachedForConfig creates a new cached Kubernetes store using informers
// This is similar to how Kubernetes operators work - it watches resources
// and maintains a local cache to avoid repeatedly hitting the API server
func NewCachedForConfig[V GenericType](cfg CachedConfig) (*CachedDynamicResource[V], error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get kubeconfig: %w", err)
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create kubernetes client: %w", err)
	}

	return newCachedStore[V](cfg, dyn)
}

// NewCachedBackend creates a backend that returns cached stores
func NewCachedBackend[V GenericType](cfg CachedConfig) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return NewCachedForConfig[V](cfg)
	}
}

// NewCachedFake creates a new cached Kubernetes store using a fake client
// This is useful for testing without requiring a real Kubernetes cluster
// The fake client still supports informers and watch functionality
// Initial objects can be provided to pre-populate the fake client before the informer starts
func NewCachedFake[V GenericType](cfg CachedConfig, initialObjects ...runtime.Object) *CachedDynamicResource[V] {
	applyDefaults(&cfg)

	gvr := schema.GroupVersionResource{
		Group:    cfg.Group,
		Version:  cfg.Version,
		Resource: cfg.Resource,
	}

	// Create fake client with custom list kinds and initial objects
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: cfg.Resource + "List"},
		initialObjects...,
	)

	cached, err := newCachedStore[V](cfg, fakeClient)
	if err != nil {
		panic(err) // Fake client should never fail to sync
	}

	return cached
}

// Stop stops the informer's watch loop
func (c *CachedDynamicResource[V]) Stop() {
	select {
	case <-c.stopCh:
		// Channel already closed
	default:
		close(c.stopCh)
	}
}

// NamespaceKeyFunc constructs a key based on whether resource is namespaced or cluster-scoped
//
//	For namespaced resources: "namespace/name"
//	For cluster-scoped resources: "name"
func NamespaceKeyFunc(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

// Read reads from the local cache instead of the API server
// This is much faster and doesn't hit the API
func (c *CachedDynamicResource[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	var obj V

	noCache, _ := store.MatchNoCache(&opts)
	if len(opts) > 0 {
		return obj, store.ErrUnsupportedOption
	}
	if noCache {
		return c.DynamicResource.Read(ctx, key, opts...)
	}

	cacheKey := NamespaceKeyFunc(c.cfg.Namespace, key)

	// Get from cache using the informer's indexer
	item, exists, err := c.informer.GetIndexer().GetByKey(cacheKey)
	if err != nil {
		return obj, err
	}

	if !exists {
		// If not found in cache, fallback to API server (optional - can also return not found error)
		return c.DynamicResource.Read(ctx, key, opts...)
	}

	// Convert unstructured to V
	unstructuredObj, ok := item.(*unstructured.Unstructured)
	if !ok {
		return obj, fmt.Errorf("cached item is not *unstructured.Unstructured")
	}

	// Use JSON marshaling instead of runtime converter to handle complex nested types
	data, err := unstructuredObj.MarshalJSON()
	if err != nil {
		return obj, err
	}

	err = json.Unmarshal(data, &obj)
	if err != nil {
		return obj, err
	}

	return obj, nil
}

// List lists from the local cache instead of the API server
func (c *CachedDynamicResource[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	noCache, _ := store.MatchNoCache(&opts)
	if len(opts) > 0 {
		return nil, store.ErrUnsupportedOption
	}
	if noCache {
		return c.DynamicResource.List(ctx, opts...)
	}

	// Get all items from cache
	items := c.informer.GetIndexer().List()

	if len(items) == 0 {
		// if nothing in cache, fallback to API server (optional - can also return empty list)
		return c.DynamicResource.List(ctx, opts...)
	}

	result := make([]store.ListItem[string, V], 0, len(items))
	for _, item := range items {
		unstructuredObj, ok := item.(*unstructured.Unstructured)
		if !ok {
			continue
		}

		var value V
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &value)
		if err != nil {
			return nil, err
		}

		result = append(result, store.ListItem[string, V]{
			Key:   unstructuredObj.GetName(),
			Value: value,
		})
	}

	return result, nil
}

// AddEventHandler allows users to register custom event handlers for Add/Update/Delete events
// This is useful for reacting to changes in real-time, just like operators do
func (c *CachedDynamicResource[V]) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return c.informer.AddEventHandler(handler)
}

// AddEventHandlerWithResyncPeriod adds event handlers with a custom resync period
func (c *CachedDynamicResource[V]) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return c.informer.AddEventHandlerWithResyncPeriod(handler, resyncPeriod)
}

// HasSynced returns true if the informer's cache has synced at least once
func (c *CachedDynamicResource[V]) HasSynced() bool {
	return c.informer.HasSynced()
}

// WaitForCacheSync waits for the cache to be synced
func (c *CachedDynamicResource[V]) WaitForCacheSync() bool {
	return cache.WaitForCacheSync(c.stopCh, c.informer.HasSynced)
}
