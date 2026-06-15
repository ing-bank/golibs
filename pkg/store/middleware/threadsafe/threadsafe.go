package threadsafe

import (
	"cmp"
	"context"
	"sync"

	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Threadsafe[string, string])(nil)

type Threadsafe[K cmp.Ordered, V any] struct {
	sync.RWMutex
	store store.Store[K, V]
}

func NewThreadsafeBuilder[K cmp.Ordered, V any]() store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return New[K, V](store)
	}
}

func New[K cmp.Ordered, V any](store store.Store[K, V]) (store.Store[K, V], error) {
	return &Threadsafe[K, V]{
		RWMutex: sync.RWMutex{},
		store:   store,
	}, nil
}

func (t *Threadsafe[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	t.Lock()
	defer t.Unlock()
	return t.store.Create(ctx, key, value, opts...)
}

func (t *Threadsafe[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	t.RLock()
	defer t.RUnlock()
	return t.store.Read(ctx, key, opts...)
}

func (t *Threadsafe[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	t.Lock()
	defer t.Unlock()
	return t.store.Update(ctx, key, value, opts...)
}

func (t *Threadsafe[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	t.Lock()
	defer t.Unlock()
	return t.store.Apply(ctx, key, value, opts...)
}

func (t *Threadsafe[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	t.Lock()
	defer t.Unlock()
	return t.store.Delete(ctx, key, opts...)
}

func (t *Threadsafe[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	t.RLock()
	defer t.RUnlock()
	return t.store.List(ctx, opts...)
}
