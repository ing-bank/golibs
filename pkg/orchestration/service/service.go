// Package service provides the Service interface. Users of this package should implement the Service for their target
// resource. Service Actions can then be transformed into workflows, e.g. to apply or delete resources. In a workflow,
// each Service's Action will be executed in order. Service is meant to be called in a workflow, executed by a
// controller.
package service

import (
	"context"

	"github.com/ing-bank/golibs/pkg/slices"
	task "github.com/ing-bank/golibs/pkg/task/workflow"
)

type Action string

const (
	ValidateAction Action = "Validate"
	ApplyAction    Action = "Apply"
	DeleteAction   Action = "Delete"
)

func (a Action) String() string {
	return string(a)
}

type Identifiable interface {
	ID() string
}

type Service[E Identifiable] interface {
	// Name returns the name of the service.
	Name() string
	// Validate validates the apply action for the given event
	Validate(ctx context.Context, event E) error
	// Apply applies the resource(s) for the given event.
	Apply(ctx context.Context, event E) error
	// Delete deletes the resource(s) for the given event.
	Delete(ctx context.Context, event E) error
}

// AsActivity is a helper function that converts a Service Action into a task.Activity for use in workflows.
func AsActivity[E Identifiable](svc Service[E], action Action) task.Activity[E] {
	if action == ValidateAction {
		return task.NewActivity(svc.Name(), svc.Validate)
	}
	if action == ApplyAction {
		return task.NewActivity(svc.Name(), svc.Apply)
	}
	if action == DeleteAction {
		return task.NewActivity(svc.Name(), svc.Delete)
	}
	panic("invalid action") // Wouldn't happen in Rust
}

// NewValidateWorkflow creates a workflow that validates all provided services in order.
// Each service's Validate method will be executed as a step in the workflow.
func NewValidateWorkflow[E Identifiable](svcs ...Service[E]) *task.Workflow[E] {
	return task.NewWorkflow(ValidateAction.String(), slices.Transform(svcs, func(item Service[E]) task.Activity[E] {
		return AsActivity(item, ValidateAction)
	}))
}

// NewApplyWorkflow creates a workflow that applies all provided services in order.
// Each service's Apply method will be executed as a step in the workflow.
func NewApplyWorkflow[E Identifiable](svcs ...Service[E]) *task.Workflow[E] {
	return task.NewWorkflow(ApplyAction.String(), slices.Transform(svcs, func(item Service[E]) task.Activity[E] {
		return AsActivity(item, ApplyAction)
	})).WithWorkflowID(func(state E) string { return state.ID() }) // Allows cancellations
}

// NewDeleteWorkflow creates a workflow that deletes all provided services in order.
// Each service's Delete method will be executed as a step in the workflow.
func NewDeleteWorkflow[E Identifiable](svcs ...Service[E]) *task.Workflow[E] {
	return task.NewWorkflow(DeleteAction.String(), slices.Transform(svcs, func(item Service[E]) task.Activity[E] {
		return AsActivity(item, DeleteAction)
	})).WithWorkflowID(func(state E) string { return state.ID() }) // Allows cancellations
}
