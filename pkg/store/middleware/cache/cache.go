// Package cache provides a composite store implementation that adds a local cache layer to a persistent store.
//
// The cache.Store type wraps two store.Store instances: a persistent backend and a local cache.
// Writes affects both the persistent store and the cache, while reads attempt to read from the cache first before
// falling back to the persistent store.

package cache

import (
	"cmp"
	"context"

	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/utilities/defaultmap"
)

var _ store.Store[string, string] = (*Store[string, string])(nil)

// Store is a composite store that wraps a persistent store and a local cache.
// Writes are performed to both the persistent store and the cache, while reads attempt to read from the cache first
// before falling back to the persistent store.
type Store[K cmp.Ordered, V any] struct {
	store store.Store[K, V] // persistent backend
	cache store.Store[K, V] // local cache
}

// New creates a new cache.Store that wraps the given persistent and local stores.
// Preferably both stores are of the same type to ensure consistent behavior, especially for options (e.g. DryRun).
// Mismatched store types may lead to inconsistent behavior, such as options being applied to one store but not the other.
func New[K cmp.Ordered, V any](persistent, local store.Store[K, V]) (store.Store[K, V], error) {
	merger, err := defaultmap.NewForStore(local)
	if err != nil {
		return nil, err
	}

	// TODO handle context more gracefully
	if err := merger.Clone(context.Background(), persistent, func(old, new V) V { return new }); err != nil {
		return nil, err
	}

	return &Store[K, V]{
		store: persistent,
		cache: local,
	}, nil
}

// Create writes the key/value to the persistent store, then to the cache.
// If the persistent store fails, the cache is not updated.
func (t *Store[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	dupOps := slices.Clone(opts)

	err := t.store.Create(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	_ = t.cache.Create(ctx, key, value, dupOps...)

	return err
}

// Read attempts to read from the cache first, then falls back to the persistent store if not found.
func (t *Store[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	skipCache, _ := MatchSkipCache(&opts)
	if skipCache {
		return t.store.Read(ctx, key, opts...)
	}

	dupOps := slices.Clone(opts)
	item, err := t.cache.Read(ctx, key, dupOps...)
	if err == nil {
		return item, nil
	}

	return t.store.Read(ctx, key, opts...)
}

// Update writes the key/value to the persistent store, then to the cache.
// If the persistent store fails, the cache is not updated.
func (t *Store[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	dupOps := slices.Clone(opts)

	err := t.store.Update(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	_ = t.cache.Update(ctx, key, value, dupOps...)

	return err
}

// Apply writes the key/value to the persistent store, then to the cache.
// If the persistent store fails, the cache is not updated.
func (t *Store[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	dupOps := slices.Clone(opts)

	err := t.store.Apply(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	_ = t.cache.Apply(ctx, key, value, dupOps...)

	return err
}

// Delete removes the key from the persistent store, then from the cache.
// If the persistent store fails, the cache is not updated.
func (t *Store[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	dupOps := slices.Clone(opts)

	err := t.store.Delete(ctx, key, opts...)
	if err != nil {
		return err
	}

	_ = t.cache.Delete(ctx, key, dupOps...)

	return err
}

// List attempts to list from the cache first, then falls back to the persistent store in case of errors.
func (t *Store[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	skipCache, _ := MatchSkipCache(&opts)

	// User wants to skip cache and due to options we cannot reliably use this to update our cache
	if skipCache && len(opts) > 0 {
		return t.store.List(ctx, opts...)
	}

	// If user wants to skip cache, but no options provided. We can use this opportunity to update our cache.
	// We fetch latest result from persistent store. We update internal cache with this information
	if skipCache {
		merger, err := defaultmap.NewForStore(t.store)
		if err != nil {
			return nil, err
		}

		// Update internal cache with persistent store data
		if err := merger.Clone(ctx, t.store, func(old, new V) V { return new }); err != nil {
			return nil, err
		}
		// From here on out we can return our updates cache data
	}

	return t.cache.List(ctx, opts...)
}
