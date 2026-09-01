package main

import (
	"context"
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/kubemock"
	"github.com/ing-bank/golibs/pkg/store/backends/kubernetes"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

type sharedFakeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              string `json:"spec,omitempty"`
	Status            string `json:"status,omitempty"`
}

func (in *sharedFakeCluster) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

type sharedFakeClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []sharedFakeCluster `json:"items"`
}

func (in *sharedFakeClusterList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	if in.Items != nil {
		out.Items = make([]sharedFakeCluster, len(in.Items))
		copy(out.Items, in.Items)
	}
	return &out
}

func TestSharedFakeStoreTriggersManagerReconcile(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "example.com", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("Cluster"), &sharedFakeCluster{})
	scheme.AddKnownTypeWithName(gv.WithKind("ClusterList"), &sharedFakeClusterList{})

	shared := kubemock.NewSharedFake(scheme)
	mgr, err := kubemock.NewManagerWithSharedFake(nil, ctrl.Options{Scheme: scheme}, shared)
	require.NoError(t, err)

	gvr := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "clusters"}
	resource := shared.ResourceClient(gvr, "default")
	store, err := kubernetes.New[*sharedFakeCluster](kubernetes.Config{
		Namespace: "default",
		Group:     "example.com",
		Version:   "v1",
		Resource:  "clusters",
	}, resource)
	require.NoError(t, err)

	informer, err := mgr.GetCache().GetInformer(ctx, &sharedFakeCluster{})
	require.NoError(t, err)

	reconcileResult := make(chan string, 2)
	reconcileErr := make(chan error, 2)
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cluster := extractSharedFakeCluster(obj)
			if cluster == nil || cluster.Name != "demo" || cluster.Status != "" {
				return
			}
			updated := cluster.DeepCopyObject().(*sharedFakeCluster)
			updated.Status = "reconciled"
			if err := mgr.GetClient().Update(ctx, updated); err != nil {
				reconcileErr <- err
				return
			}
			reconcileResult <- updated.Status
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			cluster := extractSharedFakeCluster(newObj)
			if cluster == nil || cluster.Name != "demo" || cluster.Status != "" {
				return
			}
			updated := cluster.DeepCopyObject().(*sharedFakeCluster)
			updated.Status = "reconciled"
			if err := mgr.GetClient().Update(ctx, updated); err != nil {
				reconcileErr <- err
				return
			}
			reconcileResult <- updated.Status
		},
	})
	require.NoError(t, err)

	err = store.Apply(ctx, "demo", &sharedFakeCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: "initial",
	})
	require.NoError(t, err)

	select {
	case err := <-reconcileErr:
		require.NoError(t, err)
	case status := <-reconcileResult:
		require.Equal(t, "reconciled", status)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconcile status update")
	}

	stored, err := store.Read(ctx, "demo")
	require.NoError(t, err)
	require.Equal(t, "reconciled", stored.Status)
}

func extractSharedFakeCluster(obj interface{}) *sharedFakeCluster {
	switch typed := obj.(type) {
	case *sharedFakeCluster:
		return typed
	case *unstructured.Unstructured:
		cluster := &sharedFakeCluster{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(typed.Object, cluster); err == nil {
			return cluster
		}
		return nil
	default:
		return nil
	}
}

