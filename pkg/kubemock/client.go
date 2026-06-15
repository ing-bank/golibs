// Package kubemock provides utilities for testing Kubernetes event-based listener applications.
//
// It wraps the client-go fake client to enable testing of Kubernetes resource operations
// with realistic behavior, including DryRun support and proper error handling.
//
// Primary Use Case:
// Testing event-based listener applications that watch or react to Kubernetes resource changes.
// The mock client simulates real Kubernetes API behavior without requiring a running cluster.
//
// Features:
//
//   - DryRun Support: Properly handles DryRun flag in create, update, patch, and delete operations.
//   - Error Simulation: Returns appropriate Kubernetes API errors (Conflict, NotFound) based on operation type.
//   - In-Memory Tracking: Uses ObjectTracker to maintain state of resources during tests.
//   - Fake Client: Pre-configured fake Clientset with DryRun reactor already registered.
//
// Usage:
//
//	// Create a mock client for testing
//	client := kubemock.NewFakeClient()
//
//	// Use like a normal Kubernetes client
//	pod := &corev1.Pod{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "test-pod",
//			Namespace: "default",
//		},
//		Spec: corev1.PodSpec{
//			// ...
//		},
//	}
//
//	// Create a pod
//	created, err := client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	// DryRun test - validates without persisting
//	_, err = client.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{
//		DryRun: []string{metav1.DryRunAll},
//	})
//	// err will be Conflict since pod already exists
//
package kubemock

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	testing "k8s.io/client-go/testing"
)

func DryRunReactor(tracker testing.ObjectTracker) func(action testing.Action) (handled bool, ret runtime.Object, err error) {
	return func(action testing.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case testing.CreateActionImpl:
			if opts := a.GetCreateOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				obj := a.GetObject()
				accessor, err := meta.Accessor(obj)
				if err != nil {
					return true, nil, err
				}
				ns := a.GetNamespace()
				name := accessor.GetName()
				_, err = tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err == nil {
					return true, nil, errors.NewConflict(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name, nil)
				}
				return true, a.GetObject(), nil
			}
		case testing.UpdateActionImpl:
			if opts := a.GetUpdateOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				obj := a.GetObject()
				accessor, err := meta.Accessor(obj)
				if err != nil {
					return true, nil, err
				}
				ns := a.GetNamespace()
				name := accessor.GetName()
				_, err = tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, a.GetObject(), nil
			}

		case testing.PatchActionImpl:
			if opts := a.GetPatchOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				ns := a.GetNamespace()
				name := a.GetName()
				_, err := tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, nil, nil
			}

		case testing.DeleteActionImpl:
			if opts := a.GetDeleteOptions(); len(opts.DryRun) > 0 {
				gvr := a.GetResource()
				ns := a.GetNamespace()
				name := a.GetName()
				_, err := tracker.Get(gvr, ns, name, metav1.GetOptions{})
				if err != nil {
					return true, nil, errors.NewNotFound(schema.GroupResource{Group: gvr.Group, Resource: gvr.Resource}, name)
				}
				return true, nil, nil
			}

		}
		return false, nil, nil
	}
}

// NewFakeClient returns a Kubernetes fake client with DryRun support
func NewFakeClient() *fake.Clientset {
	client := fake.NewSimpleClientset()
	// Ensure tracker is the correct type
	tracker, _ := client.Tracker().(testing.ObjectTracker)
	client.Fake.PrependReactor("*", "*", DryRunReactor(tracker))
	return client
}
