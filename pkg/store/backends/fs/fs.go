// Package fs provides a generic, file-based implementation of the store.Store interface.
//
// This package enables persistent CRUD operations on key-value pairs, where each value is stored as a JSON file
// in a specified base directory. The implementation is generic over the value type, and supports pluggable
// filesystem backends via the RWFS interface, allowing for both real and in-memory/mock filesystems.
//
// Key features:
//   - Each key is mapped to a file named <key>.json in the base directory.
//   - Implements Store: Create, Read, Update, Apply, Delete, and List operations.
//   - Thread-safe via internal locking.
//   - Extensible for testing via the RWFS interface (see NewFake).
//
// Example usage:
//   opts := fs.Options{Basepath: "/tmp"}
//   store, err := fs.New[MyType](opts)
//   // Use store.Create, store.Read, etc.
//
// For non-persistent testing, use NewFake to create a store backed by an in-memory filesystem.

package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	goerrors "errors"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Store[string])(nil)

type Options struct {
	Basepath string
	FS       RWFS // Optional, for testing/mocking
}

func (o *Options) Validate() error {
	if o.Basepath == "" {
		return goerrors.New("basepath is required")
	}
	return nil
}

type Store[V any] struct {
	fs       RWFS
	basepath string
	mu       sync.RWMutex
}

func NewBuilder[V any](opts Options) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return New[V](opts)
	}
}

func New[V any](opts Options) (store.Store[string, V], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if opts.FS == nil {
		opts.FS = NewOSFS()
	}

	if err := opts.FS.MakeDir(opts.Basepath, 0755); err != nil {
		return nil, err
	}
	return &Store[V]{fs: opts.FS, basepath: opts.Basepath}, nil
}

func NewFake[V any](opts Options) (store.Store[string, V], error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	if opts.FS == nil {
		opts.FS = NewMemFS()
	}

	return &Store[V]{fs: opts.FS, basepath: opts.Basepath}, nil
}

func (t *Store[V]) path(key string) string {
	return filepath.Join(t.basepath, fmt.Sprintf("%v.json", key))
}

func (t *Store[V]) WriteJSON(f io.Writer, value any) error {
	enc := json.NewEncoder(f)
	return enc.Encode(value)
}

func (t *Store[V]) ReadJSON(key string) (V, error) {
	f, err := t.fs.Read(t.path(key))
	if err != nil {
		return *new(V), errors.ErrNotFound
	}
	defer f.Close()

	var v V
	dec := json.NewDecoder(f)
	return v, dec.Decode(&v)
}

func (t *Store[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
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

	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	f, err := t.fs.Create(t.path(key))
	if err != nil {
		return errors.ErrConflict
	}
	defer f.Close() // TODO: handle close error

	return t.WriteJSON(f, value)
}

func (t *Store[V]) Read(_ context.Context, key string, opts ...store.Option) (V, error) {
	if len(opts) > 0 {
		return *new(V), store.ErrUnsupportedOption
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.ReadJSON(key)
}

func (t *Store[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		_, err := t.Read(ctx, key)
		return err
	}

	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	f, err := t.fs.Update(t.path(key))
	if err != nil {
		return errors.ErrNotFound
	}
	defer f.Close()

	return t.WriteJSON(f, value)
}

func (t *Store[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		return nil
	}

	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Try update first, if not found, then create
	fpath := t.path(key)
	f, err := t.fs.Update(fpath)
	if err != nil { // TODO: properly check os error, maybe
		f, err = t.fs.Create(fpath)
		if err != nil {
			return err
		}
	}
	defer f.Close()

	return t.WriteJSON(f, value)
}

func (t *Store[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if dryRun {
		_, err := t.Read(ctx, key)
		return err
	}

	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.fs.Delete(t.path(key))
}

func (t *Store[V]) List(_ context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	prefix, _ := store.MatchPrefix(&opts)
	keyOnly, _ := store.MatchListKeyOnly(&opts)
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	files, err := t.fs.ReadDir(t.basepath)
	if err != nil {
		return nil, err
	}
	var items store.ListItems[string, V]
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}
		if prefix != "" && !strings.HasPrefix(file.Name(), prefix) {
			continue
		}

		key := strings.TrimSuffix(file.Name(), ".json")
		var value V
		if !keyOnly {
			value, err = t.ReadJSON(key)
			if err != nil {
				return nil, err
			}
		}
		items = append(items, store.ListItem[string, V]{Key: key, Value: value})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	return items, nil
}
