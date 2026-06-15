package configmap

// ConfigMap is a store implementation that uses Kubernetes ConfigMaps as backend.
// Each entry is stored in a separate ConfigMap, with the key as the ConfigMap name (with optional prefix).
// The value is stored as JSON in a "payload" field in the ConfigMap data.
//
// Labels can be used to filter and organize the ConfigMaps.
// ImmutableLabels are labels that cannot be overridden by user-provided labels. When set, these labels will
// also serve as a base for label selectors when listing items. Custom selectors cannot override the
// immutable label selection.
// LabelsEnricher can be used to always add certain labels based on the object being stored.
// WithPrefix can be used to add a prefix to all ConfigMap names, useful for namespacing.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/kubemock"
	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
	v2 "k8s.io/client-go/kubernetes/typed/core/v1"

	v12 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/listers/core/v1"
)

type Config[V any] struct {
	// Namespace to store the ConfigMaps in.
	// Must match the namespace used by the lister and client.
	Namespace string

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

type ConfigMap[V any] struct {
	lister v1.ConfigMapLister
	client v2.ConfigMapInterface
	cfg    Config[V]
}

func New[V any](cfg Config[V], lister v1.ConfigMapLister, cms v2.ConfigMapInterface) (store.Store[string, V], error) {
	return &ConfigMap[V]{
		lister: lister,
		client: cms,
		cfg:    cfg,
	}, nil
}

func NewFake[V any](cfg Config[V]) (store.Store[string, V], error) {
	client := kubemock.NewFakeClient()
	lister := kubemock.NewMockConfigMapLister(client)

	return New[V](cfg, lister, client.CoreV1().ConfigMaps(cfg.Namespace))
}

func NewBackend[V any](cfg Config[V], lister v1.ConfigMapLister, cms v2.ConfigMapInterface) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return New[V](cfg, lister, cms)
	}
}

func NewFakeBackend[V any](cfg Config[V]) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return NewFake[V](cfg)
	}
}

func (c *ConfigMap[V]) BuildLabels(obj V, opts *[]store.Option) (map[string]string, error) {
	return labelstore.BuildLabels(obj, c.cfg.ImmutableLabels, c.cfg.LabelsEnricher, opts)
}

func (c *ConfigMap[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	cmLabels, err := c.BuildLabels(value, &opts)
	if err != nil {
		return err
	}
	dryRun, _ := store.MatchDryRun(&opts)
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	_, err = c.client.Create(ctx, &v12.ConfigMap{
		ObjectMeta: v13.ObjectMeta{
			Name:   key,
			Labels: cmLabels,
		},
		Data: map[string]string{"payload": string(raw)},
	}, v13.CreateOptions{DryRun: kubemock.DryRunOption(dryRun)})

	return err
}

func (c *ConfigMap[V]) Read(_ context.Context, key string, opts ...store.Option) (V, error) {
	var zero V
	if len(opts) > 0 {
		return zero, store.ErrUnsupportedOption
	}

	cm, err := c.lister.ConfigMaps(c.cfg.Namespace).Get(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return zero, errors.ErrNotFound
		}
		return zero, err
	}
	var result V
	err = json.Unmarshal([]byte(cm.Data["payload"]), &result)
	if err != nil {
		return zero, fmt.Errorf("failed to unmarshal configmap data: %w", err)
	}
	return result, nil
}

func (c *ConfigMap[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}

	cmLabels, err := c.BuildLabels(value, &opts)
	if err != nil {
		return err
	}
	dryRun, _ := store.MatchDryRun(&opts)
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}

	_, err = c.client.Update(ctx, &v12.ConfigMap{
		ObjectMeta: v13.ObjectMeta{
			Name:   key,
			Labels: cmLabels,
		},
		Data: map[string]string{"payload": string(raw)},
	}, v13.UpdateOptions{DryRun: kubemock.DryRunOption(dryRun)})
	if err != nil && apierrors.IsNotFound(err) {
		return errors.ErrNotFound
	}

	return err
}

func (c *ConfigMap[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	err := c.Update(ctx, key, value, opts...)
	if errors.IsNotFound(err) {
		err = c.Create(ctx, key, value, opts...)
	}
	return err
}

func (c *ConfigMap[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	dryRun, _ := store.MatchDryRun(&opts)
	if len(opts) > 0 {
		return store.ErrUnsupportedOption
	}
	err := c.client.Delete(ctx, key, v13.DeleteOptions{DryRun: kubemock.DryRunOption(dryRun)})
	if err != nil && apierrors.IsNotFound(err) {
		return errors.ErrNotFound
	}
	return err
}

func (c *ConfigMap[V]) List(_ context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	selector, _ := labelstore.MatchLabelSelector(&opts)
	prefix, _ := store.MatchPrefix(&opts)
	keysOnly, _ := store.MatchListKeyOnly(&opts)
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return nil, err
	}
	parsedSelector, err := labelstore.GenerateLabelSelector(c.cfg.ImmutableLabels, selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse label selector: %w", err)
	}

	maps, err := c.lister.ConfigMaps(c.cfg.Namespace).List(parsedSelector)
	if err != nil {
		return nil, err
	}

	items := []store.ListItem[string, V]{}
	for _, item := range maps {
		if prefix == "" || strings.HasPrefix(item.Name, prefix) {
			var parsed V
			if !keysOnly {
				if err := json.Unmarshal([]byte(item.Data["payload"]), &parsed); err != nil {
					return nil, fmt.Errorf("failed to unmarshal configmap data for item %s: %w", item.Name, err)
				}
			}
			items = append(items, store.ListItem[string, V]{Key: item.Name, Value: parsed})
		}
	}
	return items, nil
}
