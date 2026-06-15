package validatable

// Named middleware makes sure that object keys matches object names

import (
	"cmp"
	"context"
	"errors"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/store"
)

var ErrNameableKeyMismatch = errors.New("object key does not match object name")

type Validating[K cmp.Ordered, V config.Validatable] struct {
	store store.Store[K, V]
}

func NewNamedBuilder[K cmp.Ordered, V config.Validatable]() store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return New[K, V](store)
	}
}

func New[K cmp.Ordered, V config.Validatable](store store.Store[K, V]) (store.Store[K, V], error) {
	return &Validating[K, V]{
		store: store,
	}, nil
}

func (t *Validating[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	if err := value.Validate(); err != nil {
		return err
	}

	return t.store.Create(ctx, key, value, opts...)
}

func (t *Validating[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	return t.store.Read(ctx, key, opts...)
}

func (t *Validating[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	if err := value.Validate(); err != nil {
		return err
	}

	return t.store.Update(ctx, key, value, opts...)
}

func (t *Validating[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	if err := value.Validate(); err != nil {
		return err
	}

	return t.store.Apply(ctx, key, value, opts...)
}

func (t *Validating[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	return t.store.Delete(ctx, key, opts...)
}

func (t *Validating[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	return t.store.List(ctx, opts...)
}
