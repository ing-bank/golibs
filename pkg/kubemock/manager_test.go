package kubemock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewFakeManager(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	mgr, err := NewManager(nil, ctrl.Options{Scheme: scheme})
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string]string{"k": "v"},
	}

	require.NoError(t, mgr.GetClient().Create(context.Background(), cm))

	var stored corev1.ConfigMap
	require.NoError(t, mgr.GetClient().Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, &stored))
	require.Equal(t, "demo", stored.Name)
	require.Equal(t, "v", stored.Data["k"])
}

func TestNewFakeManager_StartDoesNotPanic(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	mgr, err := NewManager(nil, ctrl.Options{Scheme: scheme})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-time.After(100 * time.Millisecond)
		cancel()
	}()

	require.NotPanics(t, func() {
		_ = mgr.Start(ctx)
	})
}

func TestInformerHandlersReceiveFakeAddUpdateDeleteNotifications(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	mgr, err := NewManager(nil, ctrl.Options{Scheme: scheme})
	require.NoError(t, err)

	informer, err := mgr.GetCache().GetInformer(context.Background(), &corev1.ConfigMap{})
	require.NoError(t, err)

	events := make(chan string, 10)
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			events <- "add"
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			events <- "update"
		},
		DeleteFunc: func(obj interface{}) {
			events <- "delete"
		},
	})
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string]string{"k": "v"},
	}
	require.NoError(t, mgr.GetClient().Create(context.Background(), cm))
	require.Eventually(t, func() bool {
		return len(events) > 0
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "add", <-events)

	cm.Data["k"] = "updated"
	require.NoError(t, mgr.GetClient().Update(context.Background(), cm))
	require.Eventually(t, func() bool {
		return len(events) > 0
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "update", <-events)

	require.NoError(t, mgr.GetClient().Delete(context.Background(), cm))
	require.Eventually(t, func() bool {
		return len(events) > 0
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "delete", <-events)
}

type testCustomResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              string `json:"spec,omitempty"`
}

func (in *testCustomResource) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func TestNewFakeManager_UsesCustomScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	scheme.AddKnownTypes(schema.GroupVersion{Group: "custom.example.com", Version: "v1"}, &testCustomResource{})

	mgr, err := NewManager(nil, ctrl.Options{Scheme: scheme})
	require.NoError(t, err)

	obj := &testCustomResource{
		TypeMeta: metav1.TypeMeta{APIVersion: "custom.example.com/v1", Kind: "Custom"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: "ok",
	}

	require.NoError(t, mgr.GetClient().Create(context.Background(), obj))

	var stored testCustomResource
	require.NoError(t, mgr.GetClient().Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, &stored))
	require.Equal(t, "demo", stored.Name)
	require.Equal(t, "ok", stored.Spec)
}
