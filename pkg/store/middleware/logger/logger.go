package logger

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/store"
	log "github.com/sirupsen/logrus"
)

var _ store.Store[string, string] = (*Logger[string, string])(nil)

const StorePrefix = "[Store] "

type Logger[K cmp.Ordered, V any] struct {
	header string // header is the prefix for all log messages, e.g. "[Store] [MyStore] "
	store  store.Store[K, V]
}

type Config struct {
	// Name adds a prefix to all log messages, useful for distinguishing between multiple logger instances.
	Name string `json:"name"`

	// SkipStorePrefix can be used to skip the [Store] prefix in log messages
	SkipStorePrefix bool `json:"skipStorePrefix"`
}

func NewBuilder[K cmp.Ordered, V any]() store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return New[K, V](store)
	}
}

func NewBuilderForConfig[K cmp.Ordered, V any](cfg Config) store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return NewForConfig[K, V](store, cfg)
	}
}

func NewForConfig[K cmp.Ordered, V any](store store.Store[K, V], cfg Config) (store.Store[K, V], error) {
	if config.Configure(cfg) != nil {
		return nil, fmt.Errorf("failed to configure logger: %w", config.Configure(cfg))
	}
	return &Logger[K, V]{store: store, header: prepareHeader(cfg.SkipStorePrefix, cfg.Name)}, nil
}

func New[K cmp.Ordered, V any](store store.Store[K, V]) (store.Store[K, V], error) {
	return NewForConfig[K, V](store, Config{})
}

func OptionsToString(opts []store.Option) string {
	return strings.Join(slices.Transform(opts, func(item store.Option) string {
		return fmt.Sprintf("%T=%v", item, item)
	}), ",")
}

func prepareHeader(skipStorePrefix bool, name string) string {
	prefix := ""

	if !skipStorePrefix {
		prefix = StorePrefix
	}
	if name != "" {
		prefix += fmt.Sprintf("%s[%s] ", prefix, name)
	}

	return prefix
}

func (t *Logger[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)
	err := t.store.Create(ctx, key, value, opts...)
	if skip {
		return err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error(t.header + "creating store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info(t.header + "store entry created")
	return err
}

func (t *Logger[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)
	item, err := t.store.Read(ctx, key, opts...)
	if skip {
		return item, err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "error": err.Error(), "options": optionDescription}).Error(t.header + "reading store entry failed")
		return item, err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": item, "options": optionDescription}).Info(t.header + "store entry read")
	return item, err
}

func (t *Logger[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)
	err := t.store.Update(ctx, key, value, opts...)
	if skip {
		return err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error(t.header + "updating store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info(t.header + "store entry updated")
	return err
}

func (t *Logger[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)
	err := t.store.Apply(ctx, key, value, opts...)
	if skip {
		return err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error(t.header + "applying store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info(t.header + "store entry applied")
	return err
}

func (t *Logger[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)
	err := t.store.Delete(ctx, key, opts...)
	if skip {
		return err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "error": err.Error(), "options": optionDescription}).Error(t.header + "deleting store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "options": optionDescription}).Info(t.header + "store entry deleted")
	return err
}

func (t *Logger[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	skip, _ := MatchSkipLog(&opts)
	optionDescription := OptionsToString(opts)

	items, err := t.store.List(ctx, opts...)
	if skip {
		return items, err
	}
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"error": err.Error(), "options": optionDescription}).Error(t.header + "listing store entries failed")
		return items, err
	}
	log.WithContext(ctx).WithFields(log.Fields{"length": len(items), "options": optionDescription}).Info(t.header + "listing store entries succeeded")
	return items, err
}
