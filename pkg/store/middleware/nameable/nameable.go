package nameable

// Named middleware makes sure that object keys matches object names

import (
	"context"
	"errors"

	"github.com/ing-bank/golibs/pkg/store"
)

var ErrNameableKeyMismatch = errors.New("object key does not match object name")

type Named[V store.Nameable] struct {
	store store.Store[string, V]
}

func NewNamedBuilder[V store.Nameable]() store.Builder[string, V] {
	return func(store store.Store[string, V]) (store.Store[string, V], error) {
		return New[V](store)
	}
}

func New[V store.Nameable](store store.Store[string, V]) (store.Store[string, V], error) {
	return &Named[V]{
		store: store,
	}, nil
}

func (t *Named[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return ErrNameableKeyMismatch
	}

	return t.store.Create(ctx, key, value, opts...)
}

func (t *Named[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	return t.store.Read(ctx, key, opts...)
}

func (t *Named[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return ErrNameableKeyMismatch
	}

	return t.store.Update(ctx, key, value, opts...)
}

func (t *Named[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return ErrNameableKeyMismatch
	}

	return t.store.Apply(ctx, key, value, opts...)
}

func (t *Named[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	return t.store.Delete(ctx, key, opts...)
}

func (t *Named[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	return t.store.List(ctx, opts...)
}
