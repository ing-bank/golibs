package kubemock

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

type debugCustom struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              string `json:"spec,omitempty"`
	Status            string `json:"status,omitempty"`
}

func (in *debugCustom) DeepCopyObject() runtime.Object { return in }

func TestDebugShared(t *testing.T) {
	scheme := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "example.com", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("Cluster"), &debugCustom{})
	shared := NewSharedFake(scheme)
	mgr, err := NewManagerWithSharedFake(nil, ctrl.Options{Scheme: scheme}, shared)
	fmt.Println("mgr err", err)
	ctx := context.Background()
	informer, err := mgr.GetCache().GetInformer(ctx, &debugCustom{})
	fmt.Println("informer err", err)
	_, err = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) { fmt.Println("ADD fired! obj type", fmt.Sprintf("%T", obj)) }})
	fmt.Println("handler err", err)
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{"name": "demo", "namespace": "default"},
		"spec": "initial",
	}}
	fmt.Println("gvk from obj", obj.GroupVersionKind())
	_, err = shared.DynamicClient().Resource(schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "clusters"}).Namespace("default").Create(ctx, obj, metav1.CreateOptions{})
	fmt.Println("create err", err)
	fmt.Println("cache len", len(shared.cache.informers))
	for k, v := range shared.cache.informers {
		fmt.Println("informer keys", k, "handlers", len(v.handlers), "objects", len(v.objects))
	}
}

