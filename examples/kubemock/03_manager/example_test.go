package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/kubemock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// GreetingReconciler adds a "greeting" entry to every ConfigMap. It is an
// ordinary controller-runtime reconciler: nothing in it knows about kubemock.
type GreetingReconciler struct {
	Client client.Client
}

// Reconcile writes the greeting once. Skipping the write when nothing would
// change keeps the controller from reconciling forever on its own updates.
func (r *GreetingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cm corev1.ConfigMap
	if err := r.Client.Get(ctx, req.NamespacedName, &cm); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	want := "hello, " + cm.Name
	if cm.Data["greeting"] == want {
		return ctrl.Result{}, nil
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["greeting"] = want
	return ctrl.Result{}, r.Client.Update(ctx, &cm)
}

// SetupWithManager registers the reconciler. SkipNameValidation is only
// needed because examples and tests may build the same controller more than
// once in a single process.
func (r *GreetingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithOptions(controller.Options{SkipNameValidation: new(true)}).
		Complete(r)
}

// Example runs a reconciler inside a fake manager, end to end:
//
//  1. kubemock.NewManager returns a real ctrl.Manager backed by memory.
//     Objects passed to it are seeded and reconciled once the manager starts.
//  2. Register the reconciler exactly as in production.
//  3. Start the manager and write through mgr.GetClient(). Writes emit
//     informer events, which trigger reconciles.
func Example() {
	seeded := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "seeded", Namespace: "default"}}

	mgr, err := kubemock.NewManager(nil, ctrl.Options{}, seeded)
	if err != nil {
		panic(err)
	}
	if err := (&GreetingReconciler{Client: mgr.GetClient()}).SetupWithManager(mgr); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		if err := mgr.Start(ctx); err != nil {
			panic(err)
		}
	}()

	c := mgr.GetClient()
	fmt.Println("seeded:", waitForGreeting(ctx, c, "seeded"))

	created := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "created", Namespace: "default"}}
	if err := c.Create(ctx, created); err != nil {
		panic(err)
	}
	fmt.Println("created:", waitForGreeting(ctx, c, "created"))

	// Output:
	// seeded: hello, seeded
	// created: hello, created
}

// waitForGreeting polls because reconciles run in the manager's goroutines.
// In tests, prefer require.Eventually or Gomega's Eventually.
func waitForGreeting(ctx context.Context, c client.Client, name string) string {
	key := types.NamespacedName{Namespace: "default", Name: name}
	for ctx.Err() == nil {
		var cm corev1.ConfigMap
		if err := c.Get(ctx, key, &cm); err == nil && cm.Data["greeting"] != "" {
			return cm.Data["greeting"]
		}
		time.Sleep(10 * time.Millisecond)
	}
	return "<timeout>"
}

// Example_reconcileDirectly is the fastest way to unit-test a reconciler: no
// manager and no goroutines. Call Reconcile yourself against a fake client.
func Example_reconcileDirectly() {
	ctx := context.Background()
	c := kubemock.NewFakeControllerRuntimeClient(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}},
	)
	r := &GreetingReconciler{Client: c}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "app"}}
	if _, err := r.Reconcile(ctx, req); err != nil {
		panic(err)
	}

	var cm corev1.ConfigMap
	_ = c.Get(ctx, req.NamespacedName, &cm)
	fmt.Println("greeting:", cm.Data["greeting"])

	// Reconciling again changes nothing: same resourceVersion, no new event.
	before := cm.ResourceVersion
	_, _ = r.Reconcile(ctx, req)
	_ = c.Get(ctx, req.NamespacedName, &cm)
	fmt.Println("idempotent:", before == cm.ResourceVersion)

	// Output:
	// greeting: hello, app
	// idempotent: true
}
