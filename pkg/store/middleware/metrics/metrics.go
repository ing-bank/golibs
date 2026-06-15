package metrics

import (
	"cmp"
	"context"
	"time"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Config holds configuration for the metrics wrapper.
type Config struct {
	Name string // label for the store, e.g. for multi-store setups
}

var (
	calls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cache",
			Subsystem: "store",
			Name:      "golibs_store_calls_total",
			Help:      "Total number of store method calls.",
		},
		[]string{"operation", "name"},
	)
	errors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cache",
			Subsystem: "store",
			Name:      "golibs_store_errors_total",
			Help:      "Total number of store method errors.",
		},
		[]string{"operation", "name"},
	)
	duration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "cache",
			Subsystem: "store",
			Name:      "golibs_store_operation_duration_seconds",
			Help:      "Duration of store operations.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation", "name"},
	)
)

type Metrics[K cmp.Ordered, V any] struct {
	store  store.Store[K, V]
	config Config
}

func NewMetricsBuilder[K cmp.Ordered, V any](config Config) store.Builder[K, V] {
	return func(store store.Store[K, V]) (store.Store[K, V], error) {
		return NewForConfig[K, V](store, config)
	}
}

func New[K cmp.Ordered, V any](store store.Store[K, V]) (store.Store[K, V], error) {
	return NewForConfig(store, Config{Name: "default"})
}

func NewForConfig[K cmp.Ordered, V any](store store.Store[K, V], config Config) (store.Store[K, V], error) {
	return &Metrics[K, V]{
		store:  store,
		config: config,
	}, nil
}

func (m *Metrics[K, V]) observe(op string, start time.Time, err error) {
	labels := prometheus.Labels{"operation": op, "name": m.config.Name}
	calls.With(labels).Inc()
	duration.With(labels).Observe(time.Since(start).Seconds())
	if err != nil {
		errors.With(labels).Inc()
	}
}

func (m *Metrics[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	start := time.Now()
	err := m.store.Create(ctx, key, value, opts...)
	m.observe("create", start, err)
	return err
}

func (m *Metrics[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	start := time.Now()
	item, err := m.store.Read(ctx, key, opts...)
	m.observe("read", start, err)
	return item, err
}

func (m *Metrics[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	start := time.Now()
	err := m.store.Update(ctx, key, value, opts...)
	m.observe("update", start, err)
	return err
}

func (m *Metrics[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	start := time.Now()
	err := m.store.Apply(ctx, key, value, opts...)
	m.observe("apply", start, err)
	return err
}

func (m *Metrics[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	start := time.Now()
	err := m.store.Delete(ctx, key, opts...)
	m.observe("delete", start, err)
	return err
}

func (m *Metrics[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	start := time.Now()
	items, err := m.store.List(ctx, opts...)
	m.observe("list", start, err)
	return items, err
}
