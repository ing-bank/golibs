// Package defaultmap provides the DefaultMap type. It's key feature is the ability to describe how to handle a key
// collision via a user-provided function, once. After the collision function is provided the DefaultMap can be used
// without further checks for key presence, allowing the user to use DefaultMap without further error or conflict handling.
//
// DefaultMap[K, V] is a map-like structure that enables flexible handling of key collisions through user-defined resolution functions.
// It is particularly useful in scenarios where merging or updating map entries requires custom logic beyond simple overwrites.
//
// Key Features:
//   - Generic support for any cmp.Ordered key type K and any value type V.
//   - Apply: Insert or overwrite a value for a given key.
//   - Update: Insert or update a value for a given key, using a collision function to resolve conflicts.
//   - Merge: Combine two DefaultMaps, resolving key collisions with a user-provided function.
//
// Example usage:
//
//	// Create a DefaultMap with string keys and int values
//	m := defaultmap.New[string, int]()
//	_ = m.Apply("foo", 1)
//	// Update with custom collision logic
//	m.Update("foo", 2, func(old, new int) int { return old + new }) // foo = 3
//	// Merge another map, summing values on collision
//	other := defaultmap.NewForMap[string, int](map[string]int{"foo": 5, "bar": 7})
//	m.Merge(other, func(old, new int) int { return old + new }) // foo = 8, bar = 7
package defaultmap

import (
	"cmp"
	"context"
	goerrors "errors"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

// DefaultMap is a generic map type that provides Apply, Update, and Merge methods. Its
// core utility is the use of the collision function, which allows iterative updates
// to the map without first checking key presence, as showcased in the example. For
// other functions like Get or Delete, the built-in map functions can be used directly.
type DefaultMap[K cmp.Ordered, V any] struct {
	Store store.Store[K, V]
}

func New[K cmp.Ordered, V any]() *DefaultMap[K, V] {
	s, _ := memory.New[K, V]()
	return &DefaultMap[K, V]{Store: s}
}

func NewForMap[K cmp.Ordered, V any](m map[K]V) *DefaultMap[K, V] {
	return &DefaultMap[K, V]{Store: memory.NewFromMap(m)}
}

func NewForStore[K cmp.Ordered, V any](store store.Store[K, V]) (*DefaultMap[K, V], error) {
	return &DefaultMap[K, V]{
		Store: store,
	}, nil
}

// Apply sets the value for the given key, always overriding any existing value.
func (d *DefaultMap[K, V]) Apply(ctx context.Context, key K, new V) error {
	return d.Update(ctx, key, new, func(old, new V) V {
		return new
	})
}

// Update sets the value for the given key. If the key exists, the collision function is called
// with the old and new values to determine the stored value. If the key does not exist, the new value is set.
func (d *DefaultMap[K, V]) Update(ctx context.Context, key K, new V, collision func(old, new V) V) error {
	current, err := d.Store.Read(ctx, key)
	if err != nil && !goerrors.Is(err, errors.ErrNotFound) {
		return err
	}
	if err == nil {
		return d.Store.Update(ctx, key, collision(current, new))
	} else {
		return d.Store.Create(ctx, key, new)
	}
}

func (d *DefaultMap[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	return d.Store.Read(ctx, key, opts...)
}

func (d *DefaultMap[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	return d.Store.Delete(ctx, key, opts...)
}

func (d *DefaultMap[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	return d.Store.List(ctx, opts...)
}

// Merge merges 'other' into 'd'. Colliding keys will be resolved via the collision function.
func (d *DefaultMap[K, V]) Merge(ctx context.Context, other store.ReadOnlyStore[K, V], collision func(old, new V) V) error {
	items, err := store.AsMap(ctx, other)
	if err != nil {
		return err
	}

	for k, v := range items {
		if err := d.Update(ctx, k, v, collision); err != nil {
			return err
		}
	}
	return nil
}

// Clone is the same as merge, but also deletes entries from 'other', making both stores identical copies. In other
// words, it clones 'other' into 'd'. Colliding keys will be resolved via the collision function. Entries that are
// not in `other` will be removed from `d`.
func (d *DefaultMap[K, V]) Clone(ctx context.Context, other store.ReadOnlyStore[K, V], collision func(old, new V) V) error {
	theirItems, err := store.AsMap(ctx, other)
	if err != nil {
		return err
	}

	for k, v := range theirItems {
		if err := d.Update(ctx, k, v, collision); err != nil {
			return err
		}
	}

	ourItems, err := store.AsMap(ctx, d.Store)
	if err != nil {
		return err
	}
	for k, _ := range ourItems {
		if _, ok := theirItems[k]; !ok {
			if err := d.Delete(ctx, k); err != nil {
				return err
			}
		}
	}

	return nil
}
