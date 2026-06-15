package labels

import (
	"cmp"
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	"k8s.io/apimachinery/pkg/labels"
)

type LabeledData[K cmp.Ordered, V any] struct {
	Name   K                 `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Data   V                 `json:"data"`
}

type Config[V any] struct {
	// ImmutableLabels are labels that cannot be overridden by user-provided labels.
	// When set, these labels will also serve as a base for label selectors when listing items.
	// Custom selectors cannot override the immutable label selection.
	// This can be used to ensure that only ConfigMaps created by this store are listed.
	// For example, setting ImmutableLabels to {"app": "myapp"} will ensure that only ConfigMaps
	// with the label "app=myapp" are listed, regardless of any custom label selectors provided.
	ImmutableLabels map[string]string

	// LabelsEnricher can be used to always add certain labels based on the object being stored.
	// This can be useful for adding versioning or other metadata labels, and can be used in combination
	// with List selectors.
	LabelsEnricher func(obj V) (map[string]string, error)
}

type Labeller[K cmp.Ordered, V any] struct {
	cfg   Config[V]
	store store.Store[K, *LabeledData[K, V]]
}

func New[K cmp.Ordered, V any](db store.Store[K, *LabeledData[K, V]], cfg Config[V]) (store.Store[K, V], error) {
	return &Labeller[K, V]{
		cfg:   cfg,
		store: db,
	}, nil
}

func NewBackend[K cmp.Ordered, V any](db store.Store[K, *LabeledData[K, V]], cfg Config[V]) store.Backend[K, V] {
	return func() (store.Store[K, V], error) {
		return New[K, V](db, cfg)
	}
}

func (l *Labeller[K, V]) BuildLabels(obj V, opts *[]store.Option) (map[string]string, error) {
	return BuildLabels(obj, l.cfg.ImmutableLabels, l.cfg.LabelsEnricher, opts)
}

func (l *Labeller[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	enriched, err := l.store.Read(ctx, key, opts...)
	if err != nil {
		var zero V
		return zero, err
	}
	return enriched.Data, nil
}

func (l *Labeller[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	selector, _ := MatchLabelSelector(&opts)
	parsedSelector, err := GenerateLabelSelector(l.cfg.ImmutableLabels, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector: %w", err)
	}

	enriched, err := l.store.List(ctx, opts...)
	var filtered store.ListItems[K, V]
	for _, obj := range enriched {
		if parsedSelector.Matches(labels.Set(obj.Value.Labels)) {
			filtered = append(filtered, store.ListItem[K, V]{
				Key:   obj.Key,
				Value: obj.Value.Data,
			})
		}
	}

	return filtered, nil
}

func (l *Labeller[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	lbls, err := l.BuildLabels(value, &opts)
	if err != nil {
		return err
	}

	return l.store.Create(ctx, key, &LabeledData[K, V]{Name: key, Labels: lbls, Data: value}, opts...)
}

func (l *Labeller[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	lbls, err := l.BuildLabels(value, &opts)
	if err != nil {
		return err
	}

	return l.store.Update(ctx, key, &LabeledData[K, V]{Name: key, Labels: lbls, Data: value}, opts...)
}

func (l *Labeller[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	lbls, err := l.BuildLabels(value, &opts)
	if err != nil {
		return err
	}

	return l.store.Apply(ctx, key, &LabeledData[K, V]{Name: key, Labels: lbls, Data: value}, opts...)
}

func (l *Labeller[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	return l.store.Delete(ctx, key, opts...)
}
