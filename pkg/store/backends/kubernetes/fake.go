package kubernetes

import (
	"github.com/ing-bank/golibs/pkg/kubemock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/testing"
)

// gvr returns the resource the store is configured for.
func (cfg Config) gvr() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: cfg.Group, Version: cfg.Version, Resource: cfg.Resource}
}

// NewFake creates a new DynamicResource with a fake Kubernetes client for testing.
// The fake client simulates Kubernetes API behavior without requiring a real cluster.
func NewFake[V GenericType](cfg Config, opts ...Option[V]) *DynamicResource[V] {
	gvr := cfg.gvr()
	fakeClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: cfg.Resource + "List"},
	)

	// client-go's dynamic fake ignores DryRun and would persist those writes.
	// This reactor runs first on every verb and resource ("*", "*"): it answers
	// dry-run create/update/patch/delete without storing anything, returning the
	// Conflict or NotFound errors a real API server would. All other actions
	// fall through to the default tracker reactor.
	tracker, _ := fakeClient.Tracker().(testing.ObjectTracker)
	fakeClient.Fake.PrependReactor("*", "*", kubemock.DryRunReactor(tracker))

	dyn, err := New[V](cfg, fakeClient.Resource(gvr), opts...)
	if err != nil {
		panic(err)
	}
	return dyn
}

// NewFakeForCluster creates a DynamicResource on a kubemock.FakeCluster, so the
// store shares state and informer events with a fake manager and other clients
// of that cluster.
//
// As on a real cluster where the CRD is installed, the resource doesn't need a
// Go type in the cluster's scheme. If the cluster doesn't know the resource
// yet, it is installed as a namespaced resource whose Kind is cfg.Resource,
// the same Kind the store writes when a value has none (as with NewFake). A
// Go type registered without its List type is installed too, so List works.
// For a cluster-scoped resource or a different Kind, call
// cluster.InstallResource first. NewFakeForCluster panics on setup errors.
func NewFakeForCluster[V GenericType](cfg Config, cluster *kubemock.FakeCluster, opts ...Option[V]) *DynamicResource[V] {
	gvr := cfg.gvr()
	gvk, err := cluster.Client().RESTMapper().KindFor(gvr)
	if err != nil {
		gvk = gvr.GroupVersion().WithKind(cfg.Resource)
	}
	if err != nil || !cluster.Client().Scheme().Recognizes(gvk.GroupVersion().WithKind(gvk.Kind+"List")) {
		cluster.InstallResource(gvk, cfg.Resource, true)
	}
	dyn, err := New[V](cfg, cluster.DynamicClient().Resource(gvr), opts...)
	if err != nil {
		panic(err)
	}
	return dyn
}
