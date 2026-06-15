package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/kubemock"
	"github.com/ing-bank/golibs/pkg/store"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8syaml "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/testing"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ store.Store[string, *v1.ConfigMap] = &DynamicResource[*v1.ConfigMap]{}

// Config configures a Kubernetes dynamic resource store backend.
// It specifies the resource type (GVR) and namespace to operate on, plus optional immutable labels.
type Config struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Group     string `json:"group" yaml:"group"`
	Version   string `json:"version" yaml:"version"`
	Resource  string `json:"resource" yaml:"resource"`

	// ImmutableLabels are labels that cannot be overridden by user-provided labels.
	// When set, these labels will also serve as a base for label selectors when listing items.
	// Custom selectors cannot override the immutable label selection.
	// This can be used to ensure that only ConfigMaps created by this store are listed.
	// For example, setting ImmutableLabels to {"app": "myapp"} will ensure that only ConfigMaps
	// with the label "app=myapp" are listed, regardless of any custom label selectors provided.
	ImmutableLabels map[string]string `json:"immutableLabels" yaml:"immutableLabels"`
}

// LabelsEnricher is a function that dynamically adds labels based on the object being stored.
// Useful for adding versioning, metadata labels, or other computed labels.
// Can be used in combination with List selectors for filtering.
type LabelsEnricher[V any] func(obj V) (map[string]string, error)

// GenericType is a constraint for types that implement runtime.Object.
// All Kubernetes API types satisfy this constraint.
type GenericType runtime.Object

// DynamicResource is a generic Kubernetes store backend that uses the dynamic client.
// It supports CRUD operations and server-side apply for any Kubernetes resource type.
type DynamicResource[V GenericType] struct {
	client         dynamic.ResourceInterface
	cfg            Config
	labelsEnricher LabelsEnricher[V]
}

// New creates a new DynamicResource store backend with the given configuration and client.
// The client should be configured for the specific resource type (GVR) and namespace.
func New[V GenericType](cfg Config, client dynamic.ResourceInterface, opts ...Option[V]) (*DynamicResource[V], error) {
	dyn := &DynamicResource[V]{
		client: client,
		cfg:    cfg,
	}

	if err := config.ApplyOpts(dyn, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	return dyn, nil
}

// NewFake creates a new DynamicResource with a fake Kubernetes client for testing.
// The fake client simulates Kubernetes API behavior without requiring a real cluster.
func NewFake[V GenericType](cfg Config, opts ...Option[V]) *DynamicResource[V] {
	gvr := schema.GroupVersionResource{
		Group:    cfg.Group,
		Version:  cfg.Version,
		Resource: cfg.Resource,
	}

	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: cfg.Resource + "List"},
	)

	tracker, _ := fakeClient.Tracker().(testing.ObjectTracker)
	fakeClient.Fake.PrependReactor("*", "*", kubemock.DryRunReactor(tracker))

	dyn, err := New[V](cfg, fakeClient.Resource(gvr).Namespace(cfg.Namespace), opts...)
	if err != nil {
		panic(err)
	}
	return dyn
}

// NewForConfig creates a new DynamicResource using the default kubeconfig.
// It automatically discovers the cluster configuration and creates the necessary clients.
func NewForConfig[V GenericType](cfg Config) (*DynamicResource[V], error) {

	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("could not create kubernetes client: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    cfg.Group,
		Version:  cfg.Version,
		Resource: cfg.Resource,
	}
	return New[V](cfg, dyn.Resource(gvr).Namespace(cfg.Namespace))
}

// NewBackend creates a store.Backend factory function for the given configuration.
// The returned Backend can be used to create multiple store instances with the same configuration.
func NewBackend[V GenericType](cfg Config) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return NewForConfig[V](cfg)
	}
}

// toUnstructured converts a value to an unstructured object and applies labels.
// It handles JSON marshaling, decoding, and label enrichment in one step.
func (c *DynamicResource[V]) toUnstructured(value V, opts *[]store.Option) (*unstructured.Unstructured, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	dec := k8syaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, _, err = dec.Decode(raw, nil, obj)
	if err != nil {
		return nil, err
	}

	cmLabels, err := c.BuildLabels(value, opts)
	if err != nil {
		return nil, err
	}
	obj.SetLabels(cmLabels)

	return obj, nil
}

// Create creates a new Kubernetes resource.
func (c *DynamicResource[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	obj, err := c.toUnstructured(value, &opts)
	if err != nil {
		return err
	}
	if key != "" {
		obj.SetName(key)
	}
	opt, err := buildCreateOptions(opts)
	if err != nil {
		return err
	}
	return c.create(ctx, obj, CreateOption{opt.DryRun})
}

func (c *DynamicResource[V]) create(ctx context.Context, obj *unstructured.Unstructured, opt CreateOption) error {
	_, err := c.client.Create(ctx, obj, metav1.CreateOptions{
		DryRun: kubemock.DryRunOption(opt.DryRun),
	})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return fmt.Errorf("%w: %w", err, errors.ErrConflict)
		}
	}
	return err
}

// Read retrieves a Kubernetes resource by name.
func (c *DynamicResource[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	var obj V
	if len(opts) > 0 {
		return obj, store.ErrUnsupportedOption
	}

	ul, err := c.client.Get(ctx, key, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return obj, fmt.Errorf("%w: %w", err, errors.ErrNotFound)
		}
		return obj, err
	}

	raw, err := ul.MarshalJSON()
	if err != nil {
		return obj, err
	}

	err = json.Unmarshal(raw, &obj)
	if err != nil {
		return obj, err
	}

	return obj, nil
}

// Update updates an existing Kubernetes resource.
func (c *DynamicResource[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	obj, err := c.toUnstructured(value, &opts)
	if err != nil {
		return err
	}
	opt, err := buildUpdateOptions(opts)
	if err != nil {
		return err
	}
	return c.update(ctx, obj, UpdateOption{
		DryRun:          opt.DryRun,
		SubResourceOnly: opt.SubResourceOnly,
	})
}

func (c *DynamicResource[V]) update(ctx context.Context, obj *unstructured.Unstructured, opt UpdateOption) error {
	// If SubResourceOnly is set, only update the status subresource.
	// This is required for CRDs with status subresource enabled.
	if opt.SubResourceOnly {
		_, err := c.client.UpdateStatus(ctx, obj, metav1.UpdateOptions{
			DryRun: kubemock.DryRunOption(opt.DryRun),
		})
		return err
	}

	_, err := c.client.Update(ctx, obj, metav1.UpdateOptions{
		DryRun: kubemock.DryRunOption(opt.DryRun),
	})
	return err
}

// Apply performs a server-side apply operation on a Kubernetes resource.
// Creates the resource if it doesn't exist, or updates it if it does (similar to kubectl apply).
func (c *DynamicResource[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	obj, err := c.toUnstructured(value, &opts)
	if err != nil {
		return err
	}
	opt, err := buildApplyOptions(opts)
	if err != nil {
		return err
	}
	if opt.SubResourceOnly {
		_, err = c.client.ApplyStatus(ctx, key, obj, metav1.ApplyOptions{
			DryRun: kubemock.DryRunOption(opt.DryRun),
		})
		return err
	}
	return c.apply(ctx, key, obj, ApplyOption{
		DryRun:          opt.DryRun,
		ResolveConflict: opt.ResolveConflict,
		SubResourceOnly: opt.SubResourceOnly,
	})
}

// apply is the internal implementation for applying a Kubernetes resource with server-side apply.
func (c *DynamicResource[V]) apply(ctx context.Context, key string, obj *unstructured.Unstructured, opt ApplyOption) error {
	// try to get the existing resource
	existing, err := c.client.Get(ctx, key, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// resource doesn't exist, create it
			return c.create(ctx, obj, CreateOption{
				DryRun: opt.DryRun,
			})
		}
		// some other error occurred
		return err
	}
	// override the resource version to resolve conflicts if enabled
	if opt.ResolveConflict {
		obj.SetResourceVersion(existing.GetResourceVersion())
	}
	return c.update(ctx, obj, UpdateOption{
		DryRun:          opt.DryRun,
		SubResourceOnly: opt.SubResourceOnly,
	})
}

// Delete removes a Kubernetes resource by name.
func (c *DynamicResource[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	opt, err := buildDeleteOptions(opts)
	if err != nil {
		return err
	}
	return c.client.Delete(ctx, key, metav1.DeleteOptions{
		DryRun: kubemock.DryRunOption(opt.DryRun),
	})
}

// List retrieves all Kubernetes resources matching the given options.
// Supports filtering by label selector and key prefix.
// Respects immutable labels configured in the Config.
func (c *DynamicResource[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	opt, err := c.buildListOptions(opts)
	if err != nil {
		return nil, err
	}

	ul, err := c.client.List(ctx, metav1.ListOptions{
		LabelSelector: opt.LabelSelector,
	})
	if err != nil {
		return nil, err
	}

	items := make([]store.ListItem[string, V], 0, len(ul.Items))
	for _, item := range ul.Items {
		// Apply prefix filter
		if opt.Prefix != "" && !strings.HasPrefix(item.GetName(), opt.Prefix) {
			continue
		}

		var value V
		if !opt.ListKeysOnly {
			raw, err := item.MarshalJSON()
			if err != nil {
				return nil, err
			}
			err = json.Unmarshal(raw, &value)
			if err != nil {
				return nil, err
			}
		}

		items = append(items, store.ListItem[string, V]{
			Key:   item.GetName(),
			Value: value,
		})
	}

	return items, nil
}
