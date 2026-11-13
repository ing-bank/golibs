// Package v1 contains the Widget custom resource, as an API package in an
// operator project would. Real projects generate the DeepCopy methods with
// controller-gen and ship a CRD manifest alongside.
package v1

import (
	"github.com/ing-bank/golibs/pkg/kube/operator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion is the API group and version Widget is served under.
var GroupVersion = schema.GroupVersion{Group: "example.golibs.ing.com", Version: "v1"}

// AddToScheme registers Widget and WidgetList. Pass it to the runtime via
// runtime.Config.Schemes.
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &Widget{}, &WidgetList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

// Widget is a minimal custom resource. Embedding operator.Conditions as its
// status makes it satisfy operator.Obj.
type Widget struct {
	metav1.TypeMeta     `json:",inline"`
	metav1.ObjectMeta   `json:"metadata"`
	operator.Conditions `json:"status"`
}

var _ operator.Obj = (*Widget)(nil)

// DeepCopyObject implements runtime.Object.
func (w *Widget) DeepCopyObject() runtime.Object {
	out := *w
	w.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Conditions.Conditions = append([]metav1.Condition(nil), w.Conditions.Conditions...)
	return &out
}

// WidgetList is required by informers to list and watch Widgets.
type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Widget `json:"items"`
}

// DeepCopyObject implements runtime.Object.
func (l *WidgetList) DeepCopyObject() runtime.Object {
	out := *l
	l.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]Widget, len(l.Items))
	for i := range l.Items {
		out.Items[i] = *l.Items[i].DeepCopyObject().(*Widget)
	}
	return &out
}
