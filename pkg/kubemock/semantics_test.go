package kubemock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// These tests pin the Kubernetes semantics kubemock promises, independent of
// how the fake is wired internally.

var podsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

func newCoreScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	return s
}

type recordedEvent struct {
	kind string
	name string
	obj  any
}

type eventRecorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *eventRecorder) record(kind string, obj any) {
	name := ""
	if o, ok := obj.(metav1.Object); ok {
		name = o.GetName()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{kind: kind, name: name, obj: obj})
}

func (r *eventRecorder) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.kind+":"+e.name)
	}
	return out
}

func (r *eventRecorder) last() recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.events[len(r.events)-1]
}

func watchPods(t *testing.T, mgr ctrl.Manager) *eventRecorder {
	t.Helper()
	inf, err := mgr.GetCache().GetInformer(context.Background(), &corev1.Pod{})
	require.NoError(t, err)
	rec := &eventRecorder{}
	_, err = inf.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(o any) { rec.record("add", o) },
		UpdateFunc: func(_, o any) { rec.record("update", o) },
		DeleteFunc: func(o any) { rec.record("delete", o) },
	})
	require.NoError(t, err)
	return rec
}

func newClusterManager(t *testing.T, initObjs ...runtime.Object) (*FakeCluster, ctrl.Manager) {
	t.Helper()
	s := newCoreScheme(t)
	cluster := NewFakeCluster(s, initObjs...)
	mgr, err := NewManagerForFakeCluster(nil, ctrl.Options{Scheme: s}, cluster)
	require.NoError(t, err)
	return cluster, mgr
}

func demoPod(name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

func TestSemantics_InformerReplaysSeededObjects(t *testing.T) {
	_, mgr := newClusterManager(t, demoPod("seeded"))
	rec := watchPods(t, mgr)
	require.Equal(t, []string{"add:seeded"}, rec.kinds())
}

func TestSemantics_InformerEventsFollowStoredState(t *testing.T) {
	ctx := context.Background()
	_, mgr := newClusterManager(t)
	c := mgr.GetClient()
	rec := watchPods(t, mgr)

	pod := demoPod("p")
	pod.Finalizers = []string{"example.com/f"}
	require.NoError(t, c.Create(ctx, pod))
	require.NoError(t, c.Update(ctx, pod), "no-op update")
	require.Equal(t, []string{"add:p"}, rec.kinds(), "no-op write must not emit")

	require.NoError(t, c.Delete(ctx, pod))
	require.Equal(t, []string{"add:p", "update:p"}, rec.kinds(), "delete with finalizer is an update")
	stored, ok := rec.last().obj.(*corev1.Pod)
	require.True(t, ok, "events carry typed objects, got %T", rec.last().obj)
	require.NotNil(t, stored.DeletionTimestamp)

	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(pod), pod))
	pod.Finalizers = nil
	require.NoError(t, c.Update(ctx, pod))
	require.Equal(t, []string{"add:p", "update:p", "delete:p"}, rec.kinds(), "removing last finalizer deletes")
}

func TestSemantics_DeleteAllOfEmitsDeletes(t *testing.T) {
	ctx := context.Background()
	_, mgr := newClusterManager(t, demoPod("a"), demoPod("b"))
	rec := watchPods(t, mgr)
	require.NoError(t, mgr.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("default")))
	require.ElementsMatch(t, []string{"add:a", "add:b", "delete:a", "delete:b"}, rec.kinds())
}

func TestSemantics_EventsAreIsolatedFromCallerObjects(t *testing.T) {
	ctx := context.Background()
	_, mgr := newClusterManager(t)
	rec := watchPods(t, mgr)
	pod := demoPod("p")
	require.NoError(t, mgr.GetClient().Create(ctx, pod))
	pod.Name = "mutated"
	require.Equal(t, "p", rec.last().obj.(*corev1.Pod).Name)
}

func podsClient(cluster *FakeCluster) dynamic.ResourceInterface {
	return cluster.ResourceClient(podsGVR, "default")
}

func toUnstructured(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	t.Helper()
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	require.NoError(t, err)
	u := &unstructured.Unstructured{Object: m}
	u.SetAPIVersion("v1")
	u.SetKind("Pod")
	return u
}

func TestSemantics_DynamicClientSharesStateAndEvents(t *testing.T) {
	ctx := context.Background()
	cluster, mgr := newClusterManager(t)
	rec := watchPods(t, mgr)
	dyn := podsClient(cluster)

	pod := demoPod("p")
	pod.Spec.NodeName = "n1"
	require.NoError(t, mgr.GetClient().Create(ctx, pod))

	u, err := dyn.Get(ctx, "p", metav1.GetOptions{})
	require.NoError(t, err, "dynamic Get sees typed-client writes")
	require.Equal(t, pod.ResourceVersion, u.GetResourceVersion())

	require.NoError(t, unstructured.SetNestedField(u.Object, "n2", "spec", "nodeName"))
	updated, err := dyn.Update(ctx, u, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NotEqual(t, pod.ResourceVersion, updated.GetResourceVersion())
	require.EqualValues(t, 2, updated.GetGeneration())

	require.Equal(t, []string{"add:p", "update:p"}, rec.kinds())
	_, typed := rec.last().obj.(*corev1.Pod)
	require.True(t, typed, "dynamic writes emit typed objects, got %T", rec.last().obj)

	_, err = dyn.Update(ctx, u, metav1.UpdateOptions{})
	require.Error(t, err, "stale resourceVersion conflicts")
}

func TestSemantics_DynamicStatusAndMainResourceAreSeparate(t *testing.T) {
	ctx := context.Background()
	cluster, mgr := newClusterManager(t)
	dyn := podsClient(cluster)
	pod := demoPod("p")
	pod.Spec.NodeName = "n1"
	require.NoError(t, mgr.GetClient().Create(ctx, pod))

	statusOnly := pod.DeepCopy()
	statusOnly.Spec.NodeName = "ignored"
	statusOnly.Status.Phase = corev1.PodRunning
	_, err := dyn.UpdateStatus(ctx, toUnstructured(t, statusOnly), metav1.UpdateOptions{})
	require.NoError(t, err)

	var got corev1.Pod
	require.NoError(t, mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(pod), &got))
	require.Equal(t, "n1", got.Spec.NodeName, "UpdateStatus must not change spec")
	require.Equal(t, corev1.PodRunning, got.Status.Phase)

	mainOnly := got.DeepCopy()
	mainOnly.Status.Phase = corev1.PodFailed
	mainOnly.Labels = map[string]string{"x": "y"}
	_, err = dyn.Update(ctx, toUnstructured(t, mainOnly), metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(pod), &got))
	require.Equal(t, corev1.PodRunning, got.Status.Phase, "Update must not change status")
	require.Equal(t, "y", got.Labels["x"])
}

func TestSemantics_DynamicPatchTypes(t *testing.T) {
	ctx := context.Background()
	cluster, mgr := newClusterManager(t)
	dyn := podsClient(cluster)
	pod := demoPod("p")
	pod.Labels = map[string]string{"keep": "1"}
	require.NoError(t, mgr.GetClient().Create(ctx, pod))

	tests := []struct {
		name  string
		pt    types.PatchType
		patch string
		want  map[string]string
	}{
		{"merge patch is deep", types.MergePatchType, `{"metadata":{"labels":{"added":"2"}}}`, map[string]string{"keep": "1", "added": "2"}},
		{"json patch", types.JSONPatchType, `[{"op":"remove","path":"/metadata/labels/added"}]`, map[string]string{"keep": "1"}},
		{"strategic merge patch", types.StrategicMergePatchType, `{"metadata":{"labels":{"s":"3"}}}`, map[string]string{"keep": "1", "s": "3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := dyn.Patch(ctx, "p", tt.pt, []byte(tt.patch), metav1.PatchOptions{})
			require.NoError(t, err)
			require.Equal(t, tt.want, out.GetLabels())
		})
	}
}

func TestSemantics_DynamicDeleteRespectsFinalizers(t *testing.T) {
	ctx := context.Background()
	cluster, mgr := newClusterManager(t)
	pod := demoPod("p")
	pod.Finalizers = []string{"example.com/f"}
	require.NoError(t, mgr.GetClient().Create(ctx, pod))

	require.NoError(t, podsClient(cluster).Delete(ctx, "p", metav1.DeleteOptions{}))
	var got corev1.Pod
	require.NoError(t, mgr.GetClient().Get(ctx, client.ObjectKeyFromObject(pod), &got))
	require.NotNil(t, got.DeletionTimestamp)
}

func TestSemantics_DynamicDeleteCollection(t *testing.T) {
	ctx := context.Background()
	cluster, mgr := newClusterManager(t, demoPod("a"), demoPod("b"))
	rec := watchPods(t, mgr)
	require.NoError(t, podsClient(cluster).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{}))
	list, err := podsClient(cluster).List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, list.Items)
	require.ElementsMatch(t, []string{"add:a", "add:b", "delete:a", "delete:b"}, rec.kinds())
}

func TestSemantics_DynamicCreateDoesNotMutateInput(t *testing.T) {
	ctx := context.Background()
	cluster, _ := newClusterManager(t)
	in := toUnstructured(t, demoPod("p"))
	out, err := podsClient(cluster).Create(ctx, in, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Empty(t, in.GetResourceVersion())
	require.NotEmpty(t, out.GetResourceVersion())
}

func TestSemantics_ClientsetGeneration(t *testing.T) {
	ctx := context.Background()
	cs := NewFakeClient()
	pods := cs.CoreV1().Pods("default")
	_, err := pods.Create(ctx, demoPod("p"), metav1.CreateOptions{})
	require.NoError(t, err)

	tests := []struct {
		name  string
		patch string
		want  int64
	}{
		{"metadata patch keeps generation", `{"metadata":{"labels":{"a":"b"}}}`, 1},
		{"spec patch bumps generation", `{"spec":{"nodeName":"n1"}}`, 2},
		{"identical patch is a no-op", `{"spec":{"nodeName":"n1"}}`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pods.Patch(ctx, "p", types.MergePatchType, []byte(tt.patch), metav1.PatchOptions{})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Generation)
		})
	}
}

func TestSemantics_ConstructorsDoNotMutateScheme(t *testing.T) {
	s := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "custom.example.com", Version: "v1"}
	s.AddKnownTypes(gv, &testCustomResource{})
	before := len(s.AllKnownTypes())

	_, err := NewManagerForFakeCluster(nil, ctrl.Options{Scheme: s}, NewFakeCluster(s))
	require.NoError(t, err)
	_, err = NewManager(nil, ctrl.Options{Scheme: s})
	require.NoError(t, err)
	require.Len(t, s.AllKnownTypes(), before)
}

func TestSemantics_ClusterScopeIsDecidedByGroupKind(t *testing.T) {
	s := newCoreScheme(t)
	custom := schema.GroupVersion{Group: "custom.example.com", Version: "v1"}.WithKind("Node")
	s.AddKnownTypeWithName(custom, &testCustomResource{})
	mapper := defaultRESTMapperForScheme(s)

	tests := []struct {
		gvk  schema.GroupVersionKind
		want bool
	}{
		{corev1.SchemeGroupVersion.WithKind("Namespace"), true},
		{corev1.SchemeGroupVersion.WithKind("Pod"), false},
		{custom, false},
	}
	for _, tt := range tests {
		m, err := mapper.RESTMapping(tt.gvk.GroupKind(), tt.gvk.Version)
		require.NoError(t, err)
		require.Equal(t, tt.want, m.Scope.Name() == "root", tt.gvk.String())
	}
}

func TestSemantics_ManagerReconcilesUntilStable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, mgr := newClusterManager(t, demoPod("p"))
	count := 0
	var mu sync.Mutex
	err := ctrl.NewControllerManagedBy(mgr).For(&corev1.Pod{}).
		WithOptions(controller.Options{SkipNameValidation: new(true)}).
		Complete(reconcileFunc(func(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
			mu.Lock()
			count++
			mu.Unlock()
			var pod corev1.Pod
			if err := mgr.GetClient().Get(ctx, req.NamespacedName, &pod); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			pod.Status.Phase = corev1.PodRunning
			return ctrl.Result{}, mgr.GetClient().Status().Update(ctx, &pod)
		}))
	require.NoError(t, err)
	go func() { _ = mgr.Start(ctx) }()

	require.Eventually(t, func() bool {
		var pod corev1.Pod
		return mgr.GetClient().Get(ctx, client.ObjectKey{Namespace: "default", Name: "p"}, &pod) == nil && pod.Status.Phase == corev1.PodRunning
	}, 3*time.Second, 10*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	require.LessOrEqual(t, count, 3, "reconcile loop must converge")
}

type reconcileFunc func(context.Context, ctrl.Request) (ctrl.Result, error)

func (f reconcileFunc) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return f(ctx, req)
}

// An installed resource behaves like a CRD on a real cluster: the dynamic
// client works without any Go type in the scheme, reads before the first write
// see an empty collection, and the status subresource is separate.
func TestSemantics_InstalledResourceWithoutGoType(t *testing.T) {
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}
	cluster := NewFakeCluster(runtime.NewScheme())
	cluster.InstallResource(gvk, "widgets", true)
	widgets := cluster.DynamicClient().Resource(gvk.GroupVersion().WithResource("widgets")).Namespace("default")

	list, err := widgets.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, list.Items)
	_, err = widgets.Get(ctx, "a", metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "got %v", err)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("a")
	require.NoError(t, unstructured.SetNestedField(u.Object, "1", "spec", "size"))
	created, err := widgets.Create(ctx, u, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.GetGeneration())

	require.NoError(t, unstructured.SetNestedField(created.Object, "Ready", "status", "phase"))
	require.NoError(t, unstructured.SetNestedField(created.Object, "2", "spec", "size"))
	updated, err := widgets.UpdateStatus(ctx, created, metav1.UpdateOptions{})
	require.NoError(t, err)
	size, _, _ := unstructured.NestedString(updated.Object, "spec", "size")
	phase, _, _ := unstructured.NestedString(updated.Object, "status", "phase")
	require.Equal(t, "1", size, "status update must not change spec")
	require.Equal(t, "Ready", phase)

	list, err = widgets.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "Widget", list.Items[0].GetKind())
}

func TestSemantics_InstalledClusterScopedResource(t *testing.T) {
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Tenant"}
	cluster := NewFakeCluster(runtime.NewScheme())
	cluster.InstallResource(gvk, "tenants", false)

	mapping, err := cluster.Client().RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	require.NoError(t, err)
	require.Equal(t, meta.RESTScopeNameRoot, mapping.Scope.Name())

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("acme")
	tenants := cluster.DynamicClient().Resource(gvk.GroupVersion().WithResource("tenants"))
	_, err = tenants.Create(ctx, u, metav1.CreateOptions{})
	require.NoError(t, err)
	list, err := tenants.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
}

// Installing a resource also reaches the manager: informers deliver events for
// it and typed reads keep working for kinds with Go types.
func TestSemantics_InstalledResourceReachesManager(t *testing.T) {
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}
	cluster := NewFakeCluster(runtime.NewScheme())
	cluster.InstallResource(gvk, "widgets", true)
	mgr, err := NewManagerForFakeCluster(nil, ctrl.Options{}, cluster)
	require.NoError(t, err)

	inf, err := mgr.GetCache().GetInformerForKind(ctx, gvk)
	require.NoError(t, err)
	rec := &eventRecorder{}
	_, err = inf.AddEventHandler(toolscache.ResourceEventHandlerFuncs{AddFunc: func(o any) { rec.record("add", o) }})
	require.NoError(t, err)

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("a")
	_, err = cluster.DynamicClient().Resource(gvk.GroupVersion().WithResource("widgets")).Namespace("default").Create(ctx, u, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"add:a"}, rec.kinds())
}

// A Go type registered without its List type can still be listed through the
// dynamic client once installed, as on a real cluster where only the CRD
// matters. Typed List calls still need the Go List type.
func TestSemantics_InstalledTypedResourceWithoutList(t *testing.T) {
	ctx := context.Background()
	gv := schema.GroupVersion{Group: "example.com", Version: "v1"}
	s := runtime.NewScheme()
	s.AddKnownTypes(gv, &testCustomResource{})
	cluster := NewFakeCluster(s)
	cluster.InstallResource(gv.WithKind("testCustomResource"), "testcustomresources", true)

	require.NoError(t, cluster.Client().Create(ctx, &testCustomResource{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"}}))
	list, err := cluster.DynamicClient().Resource(gv.WithResource("testcustomresources")).Namespace("default").List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	require.Equal(t, "a", list.Items[0].GetName())
	require.Equal(t, "testCustomResource", list.Items[0].GetKind())
}

// Resource names that differ from the guessed plural still address the same
// objects for every operation, including Watch.
func TestSemantics_InstalledResourceWithCustomPlural(t *testing.T) {
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Database"}
	cluster := NewFakeCluster(runtime.NewScheme())
	cluster.InstallResource(gvk, "dbs", true)
	dbs := cluster.DynamicClient().Resource(gvk.GroupVersion().WithResource("dbs")).Namespace("default")

	w, err := dbs.Watch(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	defer w.Stop()

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName("orders")
	_, err = dbs.Create(ctx, u, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = dbs.Get(ctx, "orders", metav1.GetOptions{})
	require.NoError(t, err)

	select {
	case ev := <-w.ResultChan():
		require.Equal(t, "orders", ev.Object.(metav1.Object).GetName())
	case <-time.After(time.Second):
		t.Fatal("no watch event")
	}
}

func TestSemantics_DynamicRejectsMismatchedKind(t *testing.T) {
	cluster := NewFakeCluster(newCoreScheme(t))
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})
	u.SetName("x")
	_, err := cluster.DynamicClient().Resource(podsGVR).Namespace("default").Create(context.Background(), u, metav1.CreateOptions{})
	require.True(t, apierrors.IsBadRequest(err), "got %v", err)
}
