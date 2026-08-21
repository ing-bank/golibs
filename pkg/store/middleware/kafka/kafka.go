package kafka

import (
	"cmp"
	"context"
	"encoding/json"
	"time"

	"github.com/ing-bank/golibs/pkg/kafka/producer"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/store"
)

var _ store.Store[string, string] = (*Kafka[string, string])(nil)

type Config struct {
	SendDryRun             bool          `json:"send_dry_run"`
	WaitForDelivery        bool          `json:"wait_for_delivery"`
	WaitForDeliveryTimeout time.Duration `json:"wait_for_delivery_timeout"`
}

type Kafka[K cmp.Ordered, V any] struct {
	cfg   Config
	store store.Store[K, V]
	bus   *producer.Producer
}

type Action string

const ActionCreated Action = "created"
const ActionApplied Action = "applied"
const ActionUpdated Action = "updated"
const ActionDeleted Action = "deleted"

type Message[K, V any] struct {
	Key    K      `json:"key"`
	Value  V      `json:"value"`
	Action Action `json:"action"`
}

func NewBuilder[K cmp.Ordered, V any](bus *producer.Producer, optCfg ...Config) store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return New[K, V](store, bus, optCfg...)
	}
}

func New[K cmp.Ordered, V any](store store.Store[K, V], bus *producer.Producer, optCfg ...Config) (store.Store[K, V], error) {
	cfg := opt.Opt(Config{}, optCfg)
	return &Kafka[K, V]{
		cfg:   cfg,
		store: store,
		bus:   bus,
	}, nil
}

func IsDryRun(opts []store.Option) bool {
	_, dryRun := store.MatchDryRun(&opts)
	return dryRun
}

func (t *Kafka[K, V]) Produce(payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	msg := t.bus.NewMessage(raw)
	if !t.cfg.WaitForDelivery {
		return t.bus.Produce(msg)
	}

	produceOpts := []producer.ProduceOption{producer.WithWaitForDelivery(true)}
	if t.cfg.WaitForDeliveryTimeout > 0 {
		produceOpts = append(produceOpts, producer.WithWaitForDeliveryTimeout(t.cfg.WaitForDeliveryTimeout))
	}
	return t.bus.Produce(msg, produceOpts...)
}

func (t *Kafka[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	err := t.store.Create(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	if IsDryRun(opts) && !t.cfg.SendDryRun {
		return nil
	}

	return t.Produce(&Message[K, V]{
		Key:    key,
		Value:  value,
		Action: ActionCreated,
	})
}

func (t *Kafka[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	return t.store.Read(ctx, key, opts...)
}

func (t *Kafka[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	err := t.store.Update(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	if IsDryRun(opts) && !t.cfg.SendDryRun {
		return nil
	}

	return t.Produce(&Message[K, V]{
		Key:    key,
		Value:  value,
		Action: ActionUpdated,
	})
}

func (t *Kafka[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	err := t.store.Apply(ctx, key, value, opts...)
	if err != nil {
		return err
	}

	if IsDryRun(opts) && !t.cfg.SendDryRun {
		return nil
	}

	return t.Produce(&Message[K, V]{
		Key:    key,
		Value:  value,
		Action: ActionApplied,
	})
}

func (t *Kafka[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	err := t.store.Delete(ctx, key, opts...)
	if err != nil {
		return err
	}

	if IsDryRun(opts) && !t.cfg.SendDryRun {
		return nil
	}

	return t.Produce(&Message[K, V]{
		Key:    key,
		Action: ActionDeleted,
	})
}

func (t *Kafka[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	return t.store.List(ctx, opts...)
}
