package logger

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	"github.com/ing-bank/golibs/pkg/slices"
	"github.com/ing-bank/golibs/pkg/store"
	log "github.com/sirupsen/logrus"
)

var _ store.Store[string, string] = (*Logger[string, string])(nil)

type Logger[K cmp.Ordered, V any] struct {
	store store.Store[K, V]
}

func NewloggerBuilder[K cmp.Ordered, V any]() store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return New[K, V](store)
	}
}

func New[K cmp.Ordered, V any](store store.Store[K, V]) (store.Store[K, V], error) {
	return &Logger[K, V]{
		store: store,
	}, nil
}

func OptionsToString(opts []store.Option) string {
	return strings.Join(slices.Transform(opts, func(item store.Option) string {
		return fmt.Sprintf("%T=%v", item, item)
	}), ",")
}

func (t *Logger[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	optionDescription := OptionsToString(opts)
	err := t.store.Create(ctx, key, value, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error("creating store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info("store entry created")
	return err
}

func (t *Logger[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	optionDescription := OptionsToString(opts)
	item, err := t.store.Read(ctx, key, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "error": err.Error(), "options": optionDescription}).Error("reading store entry failed")
		return item, err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": item, "options": optionDescription}).Info("store entry read")
	return item, err
}

func (t *Logger[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	optionDescription := OptionsToString(opts)
	err := t.store.Update(ctx, key, value, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error("updating store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info("store entry updated")
	return err
}

func (t *Logger[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	optionDescription := OptionsToString(opts)
	err := t.store.Apply(ctx, key, value, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "error": err.Error(), "options": optionDescription}).Error("applying store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "value": value, "options": optionDescription}).Info("store entry applied")
	return err
}

func (t *Logger[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	optionDescription := OptionsToString(opts)
	err := t.store.Delete(ctx, key, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"key": key, "error": err.Error(), "options": optionDescription}).Error("deleting store entry failed")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{"key": key, "options": optionDescription}).Info("store entry deleted")
	return err
}

func (t *Logger[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	optionDescription := OptionsToString(opts)
	items, err := t.store.List(ctx, opts...)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{"error": err.Error(), "options": optionDescription}).Error("listing store entries failed")
		return items, err
	}
	log.WithContext(ctx).WithFields(log.Fields{"length": len(items), "options": optionDescription}).Info("listing store entries succeeded")
	return items, err
}
