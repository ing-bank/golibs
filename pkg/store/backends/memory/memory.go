// Package cache defines a generic cache interface for storing, retrieving,
// and managing key-value pairs with support for background maintenance operations.
package memory

import (
	"cmp"
	"context"
	goerrors "errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Memory[string, string])(nil)

// Memory is a generic, in-memory store of type V indexed by keys of type K.
type Memory[K cmp.Ordered, V any] struct {
	store map[K]V
}

func NewFromMap[K cmp.Ordered, V any](m map[K]V) store.Store[K, V] {
	return &Memory[K, V]{store: m}
}

func NewBuilder[K cmp.Ordered, V any]() store.Backend[K, V] {
	return func() (store.Store[K, V], error) {
		return New[K, V]()
	}
}

func New[K cmp.Ordered, V any]() (store.Store[K, V], error) {
	return &Memory[K, V]{store: make(map[K]V)}, nil
}

func NewOrDie[K cmp.Ordered, V any]() store.Store[K, V] {
	db, _ := New[K, V]()
	return db
}

func (t *Memory[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		_, err := t.Read(ctx, key)
		if err == nil {
			return errors.ErrConflict
		}
		if goerrors.Is(err, errors.ErrNotFound) || goerrors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}

	if _, exists := t.store[key]; exists {
		return errors.ErrConflict
	}
	t.store[key] = value
	return nil
}

func (t *Memory[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		var zero V
		return zero, err
	}

	item, exists := t.store[key]
	if !exists {
		return *new(V), errors.ErrNotFound
	}
	return item, nil
}

func (t *Memory[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		_, err := t.Read(ctx, key)
		return err
	}

	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	if _, exists := t.store[key]; !exists {
		return errors.ErrNotFound
	}
	t.store[key] = value
	return nil
}

func (t *Memory[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		return nil
	}

	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}

	t.store[key] = value
	return nil
}

func (t *Memory[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		_, err := t.Read(ctx, key)
		return err
	}

	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}

	if _, exists := t.store[key]; !exists {
		return errors.ErrNotFound
	}
	delete(t.store, key)
	return nil
}

func (t *Memory[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	prefix, _ := store.MatchPrefix(&opts)
	keysOnly, _ := store.MatchListKeyOnly(&opts)
	if prefix != "" {
		var zero K
		if _, ok := any(zero).(string); !ok {
			return nil, fmt.Errorf("match prefix option is only supported for string keys: %w", store.ErrUnsupportedOption)
		}
	}
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return nil, err
	}

	items := make([]store.ListItem[K, V], 0, len(t.store))
	for k, v := range t.store {
		if prefix != "" {
			if ks, _ := any(k).(string); !strings.HasPrefix(ks, prefix) {
				continue
			}
		}
		if keysOnly {
			v = *new(V)
		}
		items = append(items, store.ListItem[K, V]{Key: k, Value: v})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})

	return items, nil
}
