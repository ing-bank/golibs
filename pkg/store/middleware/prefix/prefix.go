package prefix

import (
	"context"
	"errors"
	"strings"

	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Prefix[string])(nil)

type Prefix[V any] struct {
	store  store.Store[string, V]
	prefix string
}

func NewBuilder[V any](prefix string) store.Builder[string, V] {
	return func(store store.Store[string, V]) (store.Store[string, V], error) {
		return New[V](store, prefix)
	}
}

func New[V any](store store.Store[string, V], prefix string) (store.Store[string, V], error) {
	if prefix == "" {
		return nil, errors.New("prefix should not be empty") // Why else use this middleware?
	}
	return &Prefix[V]{
		store:  store,
		prefix: prefix,
	}, nil
}

func (t *Prefix[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	return t.store.Create(ctx, t.prefix+key, value, opts...)
}

func (t *Prefix[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	return t.store.Read(ctx, t.prefix+key, opts...)
}

func (t *Prefix[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	return t.store.Update(ctx, t.prefix+key, value, opts...)
}

func (t *Prefix[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	return t.store.Apply(ctx, t.prefix+key, value, opts...)
}

func (t *Prefix[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	return t.store.Delete(ctx, t.prefix+key, opts...)
}

func (t *Prefix[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	if _, ok := store.MatchPrefix(&opts); ok {
		return nil, errors.New("prefix option is not supported in List when using Prefix middleware")
	}

	opts = append(opts, store.WithPrefix(t.prefix))
	items, err := t.store.List(ctx, opts...)
	if err != nil {
		return nil, err
	}

	// Strip the prefix from the keys before returning
	for i := range items {
		items[i].Key = strings.TrimPrefix(items[i].Key, t.prefix)
	}
	return items, nil
}
