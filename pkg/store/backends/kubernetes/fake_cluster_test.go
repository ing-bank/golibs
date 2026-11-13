package kubernetes

import (
	"context"
	"testing"

	"github.com/ing-bank/golibs/pkg/kubemock"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// fakeClusterWidget is deliberately not registered in any scheme: on a real
// cluster the dynamic client only needs the CRD to be installed.
type fakeClusterWidget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              struct {
		Size int `json:"size"`
	} `json:"spec"`
}

func (in *fakeClusterWidget) DeepCopyObject() runtime.Object {
	out := *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return &out
}

func TestNewFakeForCluster_UnregisteredCRD(t *testing.T) {
	ctx := context.Background()
	cluster := kubemock.NewFakeCluster(runtime.NewScheme())
	store := NewFakeForCluster[*fakeClusterWidget](Config{Namespace: "default", Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)

	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Empty(t, items)

	w := &fakeClusterWidget{}
	w.Spec.Size = 3
	require.NoError(t, store.Create(ctx, "a", w))

	got, err := store.Read(ctx, "a")
	require.NoError(t, err)
	require.Equal(t, 3, got.Spec.Size)
	require.Equal(t, "widgets", got.Kind, "like NewFake, the resource doubles as Kind")

	items, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "a", items[0].Key)
}

// A resource the cluster already knows keeps its registration, so a
// cluster-scoped install with a real Kind made beforehand is respected.
func TestNewFakeForCluster_KeepsExistingInstall(t *testing.T) {
	ctx := context.Background()
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}
	cluster := kubemock.NewFakeCluster(runtime.NewScheme())
	cluster.InstallResource(gvk, "widgets", false)
	store := NewFakeForCluster[*fakeClusterWidget](Config{Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)

	require.NoError(t, store.Create(ctx, "global", &fakeClusterWidget{TypeMeta: metav1.TypeMeta{Kind: "Widget"}}))
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "global", items[0].Key)
}

func TestNewFakeForCluster_Unstructured(t *testing.T) {
	ctx := context.Background()
	cluster := kubemock.NewFakeCluster(runtime.NewScheme())
	store := NewFakeForCluster[*unstructured.Unstructured](Config{Namespace: "default", Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)

	u := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"size": int64(1)}}}
	require.NoError(t, store.Create(ctx, "a", u))
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

// Writing a Kind the installed resource doesn't serve fails, as on a real
// cluster. Register the Go type or install the resource with that Kind.
func TestNewFakeForCluster_MismatchedKind(t *testing.T) {
	cluster := kubemock.NewFakeCluster(runtime.NewScheme())
	store := NewFakeForCluster[*fakeClusterWidget](Config{Namespace: "default", Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)
	err := store.Create(context.Background(), "a", &fakeClusterWidget{TypeMeta: metav1.TypeMeta{Kind: "Widget"}})
	require.True(t, apierrors.IsBadRequest(err), "got %v", err)
}

type fakeClusterWidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []fakeClusterWidget `json:"items"`
}

func (in *fakeClusterWidgetList) DeepCopyObject() runtime.Object {
	out := *in
	out.Items = append([]fakeClusterWidget(nil), in.Items...)
	return &out
}

// A CRD whose Go type and List type are in the scheme (as a controller needs)
// is listed through the same cluster without an install.
func TestNewFakeForCluster_RegisteredGoType(t *testing.T) {
	ctx := context.Background()
	gv := schema.GroupVersion{Group: "example.com", Version: "v1"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gv.WithKind("Widget"), &fakeClusterWidget{})
	scheme.AddKnownTypeWithName(gv.WithKind("WidgetList"), &fakeClusterWidgetList{})
	cluster := kubemock.NewFakeCluster(scheme)
	store := NewFakeForCluster[*fakeClusterWidget](Config{Namespace: "default", Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)

	require.NoError(t, store.Create(ctx, "a", &fakeClusterWidget{TypeMeta: metav1.TypeMeta{Kind: "Widget"}}))
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
}

// A Go type registered without its List type still lists, because the store
// uses the dynamic client, which on a real cluster only needs the CRD.
func TestNewFakeForCluster_RegisteredGoTypeWithoutList(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}, &fakeClusterWidget{})
	cluster := kubemock.NewFakeCluster(scheme)
	store := NewFakeForCluster[*fakeClusterWidget](Config{Namespace: "default", Group: "example.com", Version: "v1", Resource: "widgets"}, cluster)

	require.NoError(t, store.Create(ctx, "a", &fakeClusterWidget{TypeMeta: metav1.TypeMeta{Kind: "Widget"}}))
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
}
