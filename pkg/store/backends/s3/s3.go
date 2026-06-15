package s3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ing-bank/golibs/pkg/store"
)

type Config[V any] struct {
	SkipBucketCreate bool
	Bucket           string
	Encode           func(V) ([]byte, error)
	Decode           func([]byte) (V, error)
}

func (c *Config[V]) Validate() error {
	if c.Bucket == "" {
		return errors.New("bucket name is required")
	}
	return nil
}

// Store implements store.Store using an S3 backend.
type Store[V any] struct {
	client Client
	config *Config[V]
}

func NewBuilder[V any](ctx context.Context, client Client, cfg *Config[V]) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return New[V](ctx, client, cfg)
	}
}

func New[V any](ctx context.Context, client Client, cfg *Config[V]) (store.Store[string, V], error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if !cfg.SkipBucketCreate {
		err := client.CreateBucket(ctx, cfg.Bucket)
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	if cfg.Encode == nil {
		cfg.Encode = func(v V) ([]byte, error) {
			return json.Marshal(v)
		}
	}
	if cfg.Decode == nil {
		cfg.Decode = func(data []byte) (V, error) {
			var v V
			err := json.Unmarshal(data, &v)
			return v, err
		}
	}

	return &Store[V]{
		client: client,
		config: cfg,
	}, nil
}

// Create stores a new object. Fails if the object already exists. Since S3 does not support
// atomic create-if-not-exists, this does a read first to check existence. This means race conditions
// are possible.
func (s *Store[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}
	// Check if exists
	_, err := s.Read(ctx, key)
	if err == nil {
		return errors.New("object already exists")
	}
	data, err := s.config.Encode(value)
	if err != nil {
		return err
	}
	return s.client.PutObject(ctx, s.config.Bucket, fmt.Sprint(key), data)
}

func (s *Store[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	var zero V
	if len(opts) > 0 {
		return zero, store.ErrUnsupportedOption
	}
	data, err := s.client.GetObject(ctx, s.config.Bucket, fmt.Sprint(key))
	if err != nil {
		return zero, err
	}
	return s.config.Decode(data)
}

func (s *Store[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	// Check if exists
	_, err := s.Read(ctx, key)
	if err != nil {
		return errors.New("object not found")
	}
	data, err := s.config.Encode(value)
	if err != nil {
		return err
	}
	return s.client.PutObject(ctx, s.config.Bucket, fmt.Sprint(key), data)
}

func (s *Store[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	data, err := s.config.Encode(value)
	if err != nil {
		return err
	}
	return s.client.PutObject(ctx, s.config.Bucket, fmt.Sprint(key), data)
}

func (s *Store[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}
	return s.client.DeleteObject(ctx, s.config.Bucket, fmt.Sprint(key))
}

func (s *Store[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	prefix, _ := store.MatchPrefix(&opts)
	keysOnly, _ := store.MatchListKeyOnly(&opts)
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return nil, err
	}

	keys, err := s.client.ListObjects(ctx, s.config.Bucket)
	if err != nil {
		return nil, err
	}

	sort.Strings(keys)
	var items store.ListItems[string, V]
	for _, k := range keys {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}

		var val V
		if !keysOnly {
			data, err := s.client.GetObject(ctx, s.config.Bucket, k)
			if err != nil {
				continue // skip missing/corrupt TODO: is this wanted?
			}
			val, err = s.config.Decode(data)
			if err != nil {
				continue
			}
		}

		items = append(items, store.ListItem[string, V]{Key: k, Value: val})
	}

	return items, nil
}
