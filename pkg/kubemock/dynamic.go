package kubemock

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/testing"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// dynamicClient adapts a controller-runtime client to dynamic.Interface.
// Delegating instead of reimplementing CRUD keeps patch, subresource,
// finalizer and conflict semantics identical to the typed client, and writes
// emit informer events because the delegate is the eventing client.
type dynamicClient struct {
	client  ctrlclient.Client
	tracker testing.ObjectTracker
}

type dynamicResourceClient struct {
	*dynamicClient
	resource  schema.GroupVersionResource
	namespace string
}

func (c *dynamicClient) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &dynamicResourceClient{dynamicClient: c, resource: gvr}
}

// IsWatchListSemanticsUnSupported tells client-go not to use watch-list,
// which the fake tracker cannot serve.
func (c *dynamicClient) IsWatchListSemanticsUnSupported() bool {
	return true
}

func (r *dynamicResourceClient) Namespace(ns string) dynamic.ResourceInterface {
	clone := *r
	clone.namespace = ns
	return &clone
}

func (r *dynamicResourceClient) kind() (schema.GroupVersionKind, error) {
	return r.client.RESTMapper().KindFor(r.resource)
}

// object returns a copy of in (or an empty object) addressed to this resource.
func (r *dynamicResourceClient) object(in *unstructured.Unstructured, name string) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	if in != nil {
		u = in.DeepCopy()
	}
	gvk, err := r.kind()
	if err != nil {
		return nil, err
	}
	switch {
	case u.GroupVersionKind().Empty():
		u.SetGroupVersionKind(gvk)
	case u.GroupVersionKind() != gvk:
		// The API server rejects bodies whose kind doesn't match the resource.
		return nil, errors.NewBadRequest(fmt.Sprintf("the object provided is of kind %q, but resource %q serves %q", u.GroupVersionKind(), r.resource, gvk))
	}
	if name != "" {
		u.SetName(name)
	}
	if r.namespace != "" {
		u.SetNamespace(r.namespace)
	}
	return u, nil
}

func (r *dynamicResourceClient) listOptions(opts metav1.ListOptions) (*ctrlclient.ListOptions, error) {
	out := &ctrlclient.ListOptions{Namespace: r.namespace, Limit: opts.Limit, Continue: opts.Continue}
	if opts.LabelSelector != "" {
		sel, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, errors.NewBadRequest(err.Error())
		}
		out.LabelSelector = sel
	}
	if opts.FieldSelector != "" {
		sel, err := fields.ParseSelector(opts.FieldSelector)
		if err != nil {
			return nil, errors.NewBadRequest(err.Error())
		}
		out.FieldSelector = sel
	}
	return out, nil
}

func deleteOptions(opts metav1.DeleteOptions) ctrlclient.DeleteOptions {
	return ctrlclient.DeleteOptions{
		GracePeriodSeconds: opts.GracePeriodSeconds,
		Preconditions:      opts.Preconditions,
		PropagationPolicy:  opts.PropagationPolicy,
		DryRun:             opts.DryRun,
	}
}

func unsupportedSubresource(subresources []string) error {
	return errors.NewMethodNotSupported(schema.GroupResource{}, fmt.Sprintf("subresource %v", subresources))
}

func (r *dynamicResourceClient) Create(ctx context.Context, obj *unstructured.Unstructured, opts metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(subresources) > 0 {
		return nil, unsupportedSubresource(subresources)
	}
	u, err := r.object(obj, "")
	if err != nil {
		return nil, err
	}
	return r.written(ctx, u, opts.DryRun, r.client.Create(ctx, u, &ctrlclient.CreateOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager}))
}

func (r *dynamicResourceClient) Update(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	u, err := r.object(obj, "")
	if err != nil {
		return nil, err
	}
	update := ctrlclient.UpdateOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager}
	if len(subresources) > 0 {
		return r.written(ctx, u, opts.DryRun, r.client.SubResource(subresources[0]).Update(ctx, u, &ctrlclient.SubResourceUpdateOptions{UpdateOptions: update}))
	}
	return r.written(ctx, u, opts.DryRun, r.client.Update(ctx, u, &update))
}

func (r *dynamicResourceClient) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, opts metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return r.Update(ctx, obj, opts, "status")
}

func (r *dynamicResourceClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions, subresources ...string) error {
	if len(subresources) > 0 {
		return unsupportedSubresource(subresources)
	}
	u, err := r.object(nil, name)
	if err != nil {
		return err
	}
	del := deleteOptions(opts)
	return r.client.Delete(ctx, u, &del)
}

func (r *dynamicResourceClient) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	u, err := r.object(nil, "")
	if err != nil {
		return err
	}
	list, err := r.listOptions(listOpts)
	if err != nil {
		return err
	}
	return r.client.DeleteAllOf(ctx, u, &ctrlclient.DeleteAllOfOptions{ListOptions: *list, DeleteOptions: deleteOptions(opts)})
}

func (r *dynamicResourceClient) Get(ctx context.Context, name string, _ metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	if len(subresources) > 0 {
		return nil, unsupportedSubresource(subresources)
	}
	u, err := r.object(nil, name)
	if err != nil {
		return nil, err
	}
	return result(u, r.client.Get(ctx, ctrlclient.ObjectKeyFromObject(u), u))
}

func (r *dynamicResourceClient) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	gvk, err := r.kind()
	if err != nil {
		return nil, err
	}
	listOpts, err := r.listOptions(opts)
	if err != nil {
		return nil, err
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
	if err := r.client.List(ctx, list, listOpts); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *dynamicResourceClient) Watch(_ context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	if r.tracker == nil {
		return nil, errors.NewMethodNotSupported(r.resource.GroupResource(), "watch")
	}
	gvk, err := r.kind()
	if err != nil {
		return nil, err
	}
	// The controller-runtime fake stores objects under the plural guessed from
	// the kind, not under the resource name the caller used.
	stored, _ := meta.UnsafeGuessKindToResource(gvk)
	return r.tracker.Watch(stored, r.namespace, opts)
}

func (r *dynamicResourceClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	u, err := r.object(nil, name)
	if err != nil {
		return nil, err
	}
	patch := ctrlclient.PatchOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager, Force: opts.Force}
	if len(subresources) > 0 {
		return r.written(ctx, u, opts.DryRun, r.client.SubResource(subresources[0]).Patch(ctx, u, ctrlclient.RawPatch(pt, data), &ctrlclient.SubResourcePatchOptions{PatchOptions: patch}))
	}
	return r.written(ctx, u, opts.DryRun, r.client.Patch(ctx, u, ctrlclient.RawPatch(pt, data), &patch))
}

func (r *dynamicResourceClient) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	u, err := r.object(obj, name)
	if err != nil {
		return nil, err
	}
	data, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return r.Patch(ctx, name, types.ApplyPatchType, data, metav1.PatchOptions{DryRun: opts.DryRun, FieldManager: opts.FieldManager, Force: &opts.Force}, subresources...)
}

func (r *dynamicResourceClient) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return r.Apply(ctx, name, obj, opts, "status")
}

// result mirrors the real dynamic client, which returns no object on error.
func result(u *unstructured.Unstructured, err error) (*unstructured.Unstructured, error) {
	if err != nil {
		return nil, err
	}
	return u, nil
}

// written returns the stored object after a write, like the API server does.
// The controller-runtime fake only copies some server-set fields (such as
// generation) back into unstructured objects. Dry-run writes return u as is.
func (r *dynamicResourceClient) written(ctx context.Context, u *unstructured.Unstructured, dryRun []string, err error) (*unstructured.Unstructured, error) {
	if err != nil {
		return nil, err
	}
	if len(dryRun) > 0 {
		return u, nil
	}
	return result(u, r.client.Get(ctx, ctrlclient.ObjectKeyFromObject(u), u))
}
