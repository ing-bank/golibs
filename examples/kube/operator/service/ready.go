// Package service holds the operator logic: one operator.Service per concern.
// The runtime reconciles each Widget by running the services in order.
package service

import (
	"context"

	v1 "github.com/ing-bank/golibs/examples/kube/operator/api/v1"
	"github.com/ing-bank/golibs/pkg/kube/operator"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionReady is set once a Widget has been reconciled.
const ConditionReady = "Ready"

// Ready marks a Widget as ready. A real service would hold its dependencies
// (clients, config) as fields, create or update dependent resources in Apply
// and clean them up in Delete.
type Ready struct{}

var _ operator.Service[*v1.Widget] = (*Ready)(nil)

// NewReady returns a Ready service.
func NewReady() *Ready {
	return &Ready{}
}

// GetName identifies the service in logs and errors.
func (*Ready) GetName() string {
	return "ReadyService"
}

// Apply sets the Ready condition for the Widget's current generation.
func (*Ready) Apply(_ context.Context, w *v1.Widget) error {
	meta.SetStatusCondition(w.GetConditions(), metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "WidgetReady",
		ObservedGeneration: w.GetGeneration(),
	})
	return nil
}

// Delete has nothing to clean up.
func (*Ready) Delete(context.Context, *v1.Widget) error {
	return nil
}

// ReadyStatus returns the status of the Ready condition, or "<unset>".
func ReadyStatus(w *v1.Widget) string {
	if c := meta.FindStatusCondition(w.Conditions.Conditions, ConditionReady); c != nil {
		return string(c.Status)
	}
	return "<unset>"
}
