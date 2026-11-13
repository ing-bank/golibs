package main

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/kubemock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Example shows the informer events kubemock produces. Events reflect what
// was stored, as a real watch does, and are delivered synchronously: when a
// write returns, its handlers have already run. The manager doesn't even need
// to be started.
func Example() {
	ctx := context.Background()
	mgr, err := kubemock.NewManager(nil, ctrl.Options{})
	if err != nil {
		panic(err)
	}
	c := mgr.GetClient()

	informer, _ := mgr.GetCache().GetInformer(ctx, &corev1.ConfigMap{})
	_, _ = informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) { fmt.Println("  event: add") },
		UpdateFunc: func(_, obj any) {
			cm := obj.(*corev1.ConfigMap)
			fmt.Printf("  event: update (deleting=%v, finalizers=%v)\n", cm.DeletionTimestamp != nil, cm.Finalizers)
		},
		DeleteFunc: func(obj any) { fmt.Println("  event: delete") },
	})

	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}}

	fmt.Println("create")
	_ = c.Create(ctx, cm)

	fmt.Println("update without changes")
	_ = c.Update(ctx, cm)

	fmt.Println("add finalizer")
	controllerutil.AddFinalizer(cm, "example.com/cleanup")
	_ = c.Update(ctx, cm)

	fmt.Println("delete")
	_ = c.Delete(ctx, cm)

	fmt.Println("remove finalizer")
	_ = c.Get(ctx, client.ObjectKeyFromObject(cm), cm)
	controllerutil.RemoveFinalizer(cm, "example.com/cleanup")
	_ = c.Update(ctx, cm)

	// Output:
	// create
	//   event: add
	// update without changes
	// add finalizer
	//   event: update (deleting=false, finalizers=[example.com/cleanup])
	// delete
	//   event: update (deleting=true, finalizers=[example.com/cleanup])
	// remove finalizer
	//   event: delete
}
