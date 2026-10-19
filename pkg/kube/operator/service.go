package operator

import (
	"context"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/retry"
	"github.com/ing-bank/golibs/pkg/slices"
	wf "github.com/ing-bank/golibs/pkg/task/workflow"
)

// Service is a service-like reconciler step. Each service is responsible for a
// part of the reconciliation and must be idempotent and non-blocking.
type Service[T Obj] interface {
	GetName() string
	Apply(ctx context.Context, item T) error
	Delete(ctx context.Context, item T) error
}

type (
	Step[T Obj]   = func(ctx context.Context, item T) error
	Action[T Obj] = func(svc Service[T]) Step[T]
)

func ActionApply[T Obj](svc Service[T]) Step[T]  { return svc.Apply }
func ActionDelete[T Obj](svc Service[T]) Step[T] { return svc.Delete }

type Services[T Obj] []Service[T]

func (s Services[T]) asWorkflow(name string, action Action[T], wrapper func(Service[T], Action[T]) Step[T]) *wf.Workflow[T] {
	chain := slices.Transform(s, func(service Service[T]) wf.Activity[T] {
		return wf.NewActivity(service.GetName(), wrapper(service, action))
	})
	return wf.NewWorkflow(name, chain).WithRetryPolicy(retry.NoRetry).WithEnableLogging(true)
}

func NewAnonService[T Obj](name string, opts ...config.Option[*AnonService[T]]) (Service[T], error) {
	svc := &AnonService[T]{name: name}
	return svc, config.ApplyOptions[*AnonService[T]](svc, opts...)
}

type AnonService[T any] struct {
	name   string
	apply  func(ctx context.Context, obj T) error
	delete func(ctx context.Context, obj T) error
}

func WithApply[T Obj](apply Step[T]) config.Option[*AnonService[T]] {
	return func(svc *AnonService[T]) error {
		svc.apply = apply
		return nil
	}
}

func WithDelete[T Obj](delete Step[T]) config.Option[*AnonService[T]] {
	return func(svc *AnonService[T]) error {
		svc.delete = delete
		return nil
	}
}

func (s *AnonService[T]) GetName() string                         { return s.name }
func (s *AnonService[T]) Apply(ctx context.Context, obj T) error  { return s.apply(ctx, obj) }
func (s *AnonService[T]) Delete(ctx context.Context, obj T) error { return s.delete(ctx, obj) }
