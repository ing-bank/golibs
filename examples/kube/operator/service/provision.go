// Package service holds the operator logic: one operator.Service per concern.
// The runtime reconciles each Widget by running the services in order.
//
// Services only do their own work; they do not manage status conditions. The
// operator.Reconciler records a condition per service (Type = GetName(), True
// on success, False with the error as reason on failure) and sets the
// aggregate "Ready" condition once all services succeeded.
package service

import (
	"context"

	v1 "github.com/ing-bank/golibs/examples/kube/operator/api/v1"
	"github.com/ing-bank/golibs/pkg/kube/operator"
)

// Provision stands in for a service that manages resources owned by a Widget.
// A real service would hold its dependencies (clients, config) as fields,
// create or update dependent resources in Apply and clean them up in Delete.
type Provision struct{}

var _ operator.Service[*v1.Widget] = (*Provision)(nil)

// NewProvision returns a Provision service.
func NewProvision() *Provision {
	return &Provision{}
}

// GetName identifies the service in logs and is used as its condition type.
func (*Provision) GetName() string {
	return "Provisioned"
}

// Apply would create or update dependent resources; returning an error marks
// the "Provisioned" and "Ready" conditions False and requeues the Widget.
func (*Provision) Apply(context.Context, *v1.Widget) error {
	return nil
}

// Delete would clean up dependent resources before the finalizer is removed.
func (*Provision) Delete(context.Context, *v1.Widget) error {
	return nil
}
