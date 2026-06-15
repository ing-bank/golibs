// Package store contains generic interfaces and implementations for key-value stores.
//
// There are three important concepts in this package:
// - Backends: these are store implementations that provide the actual storage mechanism (e.g., in-memory, database, etc.)
// - Middleware: these are wrappers around stores that add additional functionality (e.g., metrics, logging, etc.)
// - Options: these are functional options that can be passed to store methods to modify their behavior (e.g., dry-run, labels, etc.)
// The package provides a way to compose these concepts together to create a final store that meets the application's needs.
//
// Generally, a store is build by calling the store.New function with a backend and optional middleware builders. To make
// creating stores easier, the package provides Builder and Backend initialization functions. See the examples for more details.
//
// Backends need to make sure all options are supported and return ErrUnsupportedOption if an option is not supported.
package store

import (
	"cmp"
	"context"
)

type ReadOnlyStore[K cmp.Ordered, V any] interface {
	Read(ctx context.Context, key K, opts ...Option) (V, error)
	List(ctx context.Context, opts ...Option) (ListItems[K, V], error)
}

type Store[K cmp.Ordered, V any] interface {
	ReadOnlyStore[K, V]
	Create(ctx context.Context, key K, value V, opts ...Option) error
	Update(ctx context.Context, key K, value V, opts ...Option) error
	Apply(ctx context.Context, key K, value V, opts ...Option) error
	Delete(ctx context.Context, key K, opts ...Option) error
}

// Nameable is an interface for types that have a name.
type Nameable interface {
	GetName() string
}

type ListItem[K cmp.Ordered, V any] struct {
	Key   K `json:"key"`
	Value V `json:"value,omitempty"`
}

type ListItems[K cmp.Ordered, V any] []ListItem[K, V]

func AsMap[K cmp.Ordered, V any](ctx context.Context, store ReadOnlyStore[K, V], listOptions ...Option) (map[K]V, error) {
	items, err := store.List(ctx, listOptions...)
	if err != nil {
		return nil, err
	}
	return items.AsMap(), nil
}

func (items *ListItems[K, V]) AsMap() map[K]V {
	m := make(map[K]V, len(*items))
	for _, item := range *items {
		m[item.Key] = item.Value
	}
	return m
}

func Reset[K cmp.Ordered, V any](ctx context.Context, store Store[K, V]) error {
	// List all items
	items, err := store.List(ctx)
	if err != nil {
		return err
	}
	// Delete each item
	for _, item := range items {
		if err := store.Delete(ctx, item.Key); err != nil {
			return err
		}
	}
	return nil
}

type Builder[K cmp.Ordered, V any] func(Store[K, V]) (Store[K, V], error)

type Backend[K cmp.Ordered, V any] func() (Store[K, V], error)

func New[K cmp.Ordered, V any](
	outer Backend[K, V],
	middleware ...Builder[K, V],
) (Store[K, V], error) {
	// TODO: differentiate between frontend, middle and backend builders
	store, err := outer()
	if err != nil {
		return nil, err
	}
	for _, b := range middleware {
		store, err = b(store)
		if err != nil {
			return nil, err
		}
	}
	return store, err
}
