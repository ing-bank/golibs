// Package timed provides a generic, thread-safe cache implementation
// where each item is associated with a timestamp. Items are automatically
// expired and removed based on a configurable sync period. Accessing a cache
// entry resets its timestamp, extending its lifetime.
package timed

import (
	"cmp"
	"context"
	"time"

	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Timed[string, string])(nil)

// Cache is a generic interface for a timed cache.
type Cache[K cmp.Ordered, V any] interface {
	store.Store[K, V]
	ReadEntry(ctx context.Context, key K, opts ...store.Option) (CacheItem[V], error)
	Run(ctx context.Context, opts ...store.Option) error
	DeleteItemsOlderThan(ctx context.Context, maxAge time.Duration) error
}

type Timed[K cmp.Ordered, V any] struct {
	store store.Store[K, CacheItem[V]]
	cfg   *Config
}

// NewForBuilders creates a new Timed cache using the provided cache builders.
// Example: NewForBuilders(cfg, memory.New, threadsafe.New)
func NewForBuilders[K cmp.Ordered, V any](cfg *Config, outer func() (store.Store[K, CacheItem[V]], error), others ...store.Builder[K, CacheItem[V]]) (*Timed[K, V], error) {
	store, err := store.New[K, CacheItem[V]](outer, others...)
	if err != nil {
		return nil, err
	}
	return New(cfg, store)
}

// New creates a new Timed cache with the given configuration and backend store.
// The backend store must be of type store.Store[K, CacheItem[V]] as this timed cache
// relies on CacheItem to manage timestamps and expiration. Use the NewForBuilders function
// for easier construction with common store types.
func New[K cmp.Ordered, V any](cfg *Config, backend store.Store[K, CacheItem[V]]) (*Timed[K, V], error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Timed[K, V]{
		store: backend,
		cfg:   cfg,
	}, nil
}

func (t *Timed[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	return t.store.Create(ctx, key, NewCacheItem(value), opts...)
}

func (t *Timed[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	// TODO: we could delete the provided key here if it is expired

	item, err := t.store.Read(ctx, key, opts...)
	if err != nil {
		var zero V
		return zero, err
	}

	if t.cfg.RefreshAgeOnRead {
		item.RefreshTimestamp()
		err = t.store.Update(ctx, key, item)
	}
	return item.Value, err
}

func (t *Timed[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	return t.store.Update(ctx, key, NewCacheItem(value), opts...)
}

func (t *Timed[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	return t.store.Apply(ctx, key, NewCacheItem(value), opts...)
}

func (t *Timed[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	return t.store.Delete(ctx, key, opts...)
}

func (t *Timed[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	items, err := t.store.List(ctx, opts...)
	if err != nil {
		return nil, err
	}

	result := make([]store.ListItem[K, V], 0, len(items))
	for _, item := range items {
		result = append(result, store.ListItem[K, V]{
			Key:   item.Key,
			Value: item.Value.Value,
		})
	}
	return result, nil
}

// Run starts the background maintenance tasks for the cache, removing expired items.
// It is possible to provide a callback function via WithDoFunc option, which will be called
// on each sync period, before removing expired items.
func (t *Timed[K, V]) Run(ctx context.Context, opts ...store.Option) error {
	do, callDo := matchWithFunc(&opts)
	ticker := time.NewTicker(t.cfg.SyncPeriod.Duration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if callDo {
				if err := do(ctx); err != nil {
					return err
				}
			}
			if err := t.DeleteItemsOlderThan(ctx, t.cfg.MaxAge.Duration); err != nil {
				return err
			}
		}
	}
}

func (t *Timed[K, V]) ReadEntry(ctx context.Context, key K, opts ...store.Option) (CacheItem[V], error) {
	return t.store.Read(ctx, key, opts...)
}

func (t *Timed[K, V]) DeleteItemsOlderThan(ctx context.Context, maxAge time.Duration) error {
	items, err := t.store.List(ctx)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.Value.IsExpired(maxAge) {
			if err := t.store.Delete(ctx, item.Key); err != nil {
				return err
			}
		}
	}
	return nil
}
