package kubemock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              string `json:"spec,omitempty"`
}

func (in *testManagedCluster) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

type testManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []testManagedCluster `json:"items"`
}

func (in *testManagedClusterList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	if in.Items != nil {
		out.Items = make([]testManagedCluster, len(in.Items))
		copy(out.Items, in.Items)
	}
	return &out
}

type testClusterPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              string `json:"spec,omitempty"`
}

func (in *testClusterPool) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

type testClusterPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []testClusterPool `json:"items"`
}

func (in *testClusterPoolList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	if in.Items != nil {
		out.Items = make([]testClusterPool, len(in.Items))
		copy(out.Items, in.Items)
	}
	return &out
}

func makeUnstructured(gv schema.GroupVersion, kind, name, namespace, spec string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": gv.String(),
		"kind":       kind,
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}}
}

func TestSharedFakeDynamicClient_ListCustomCRDObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	gv := schema.GroupVersion{Group: "ichp.ing.net", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("ManagedCluster"), &testManagedCluster{})
	scheme.AddKnownTypeWithName(gv.WithKind("ManagedClusterList"), &testManagedClusterList{})

	shared := NewSharedFake(scheme)
	resource := shared.ResourceClient(schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: "managedclusters"}, "default")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := resource.Create(ctx, makeUnstructured(gv, "ManagedCluster", "mc-"+string(rune('0'+i)), "default", "spec-"+string(rune('0'+i))), metav1.CreateOptions{})
		require.NoError(t, err)
	}

	list, err := resource.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, "ManagedClusterList", list.GetKind())
	require.Len(t, list.Items, 3)
	require.Equal(t, "mc-0", list.Items[0].GetName())
	require.Equal(t, "mc-1", list.Items[1].GetName())
	require.Equal(t, "mc-2", list.Items[2].GetName())

	_, err = resource.Apply(ctx, "mc-1", makeUnstructured(gv, "ManagedCluster", "mc-1", "default", "patched"), metav1.ApplyOptions{FieldManager: "kubemock-test"})
	require.NoError(t, err)

	list, err = resource.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, list.Items, 3)
	require.Equal(t, "patched", list.Items[1].Object["spec"])
}

func TestSharedFakeDynamicClient_SupportsMultipleCRDsAndBuiltins(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	managedGV := schema.GroupVersion{Group: "ichp.ing.net", Version: "v1"}
	scheme.AddKnownTypeWithName(managedGV.WithKind("ManagedCluster"), &testManagedCluster{})
	scheme.AddKnownTypeWithName(managedGV.WithKind("ManagedClusterList"), &testManagedClusterList{})

	poolGV := schema.GroupVersion{Group: "clusters.example.com", Version: "v1"}
	scheme.AddKnownTypeWithName(poolGV.WithKind("ClusterPool"), &testClusterPool{})
	scheme.AddKnownTypeWithName(poolGV.WithKind("ClusterPoolList"), &testClusterPoolList{})

	shared := NewSharedFake(scheme)
	ctx := context.Background()

	managed := shared.ResourceClient(schema.GroupVersionResource{Group: managedGV.Group, Version: managedGV.Version, Resource: "managedclusters"}, "default")
	for i := 0; i < 2; i++ {
		_, err := managed.Create(ctx, makeUnstructured(managedGV, "ManagedCluster", "mc-"+string(rune('0'+i)), "default", "cluster-"+string(rune('0'+i))), metav1.CreateOptions{})
		require.NoError(t, err)
	}

	pools := shared.ResourceClient(schema.GroupVersionResource{Group: poolGV.Group, Version: poolGV.Version, Resource: "clusterpools"}, "default")
	_, err := pools.Create(ctx, makeUnstructured(poolGV, "ClusterPool", "pool-a", "default", "ready"), metav1.CreateOptions{})
	require.NoError(t, err)

	configmaps := shared.ResourceClient(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, "default")
	_, err = configmaps.Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "cm-a",
			"namespace": "default",
		},
		"data": map[string]interface{}{"purpose": "demo"},
	}}, metav1.CreateOptions{})
	require.NoError(t, err)

	managedList, err := managed.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, managedList.Items, 2)
	require.Equal(t, "ManagedClusterList", managedList.GetKind())

	poolList, err := pools.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, poolList.Items, 1)
	require.Equal(t, "ClusterPoolList", poolList.GetKind())

	cmList, err := configmaps.List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, cmList.Items, 1)
	require.Equal(t, "ConfigMapList", cmList.GetKind())
}
