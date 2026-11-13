package kubemock

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"
)

// DryRunOption is a helper function that sets the appropriate Kubernetes dry run option. Example:
//
//	metav1.CreateOptions{
//	  DryRun: kube.DryRunOption(isDryRun),
//	})
func DryRunOption(isDryRun bool) []string {
	if !isDryRun {
		return nil
	}
	return []string{"All"}
}

// DryRunReactor returns a client-go fake reactor that answers dry-run writes
// without persisting them, returning the Conflict/NotFound errors the API
// server would.
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
