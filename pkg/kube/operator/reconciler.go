package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/kube/log"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/slices"
	wf "github.com/ing-bank/golibs/pkg/task/workflow"
	"github.com/ing-bank/golibs/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Obj is the constraint for reconciled resources. T is the pointer type
// (e.g. *MyCR); see NewObject and ADR-0004 for how fresh instances are created.
type Obj interface {
	client.Object
	StatusConditions
}

// Reconciler manages a resource lifecycle and delegates each step to a
// list of backend services. It intentionally avoids returning regular errors for
// transient states and uses requeue results instead.
type Reconciler[T Obj] struct {
	Client client.Client

	applyWf  *wf.Workflow[T]
	deleteWf *wf.Workflow[T]

	cfg     Config[T]
	backoff retryBackoff
}

// Config configures finalization and non-blocking retries of failed workflows.
type Config[T Obj] struct {
	FinalizerName string        `json:"finalizerName"`
	Backoff       BackoffConfig `json:"backoff,omitempty"`
	Client        client.Client `json:"-"` // Mandatory
	Services      []Service[T]  `json:"-"` // Mandatory
}

// ApplyDefaults fills unset configuration fields.
func (c *Config[T]) ApplyDefaults() {
	if c.FinalizerName == "" {
		c.FinalizerName = "finalizer"
	}
	c.Backoff.ApplyDefaults()
}

// Validate checks the backoff configuration after defaults are applied.
func (c *Config[T]) Validate() error {
	if len(c.Services) == 0 {
		return fmt.Errorf("at least one service must be provided")
	}
	if c.Client == nil {
		return errors.New("client must be provided")
	}

	return c.Backoff.Validate()
}

func New[T Obj](client client.Client, services []Service[T], opts ...config.Option[*Config[T]]) (*Reconciler[T], error) {
	cfg := Config[T]{
		Client:   client,
		Services: services,
	}
	if err := config.ApplyOptions(&cfg, opts...); err != nil {
		return nil, fmt.Errorf("invalid option: %w", err)
	}
	return NewForConfig(cfg)
}

// NewForConfig creates a Reconciler for resource type T that runs services in
// order on apply and delete. It returns an error if cfg is invalid after applying defaults.
func NewForConfig[T Obj](cfg Config[T]) (*Reconciler[T], error) {
	if err := config.Configure(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Reconciler[T]{
		Client:   cfg.Client,
		applyWf:  Services[T](cfg.Services).asWorkflow("apply", ActionApply[T], StatusWrapper[T]),
		deleteWf: Services[T](cfg.Services).asWorkflow("delete", ActionDelete[T], StatusWrapper[T]),
		cfg:      cfg,
		backoff:  retryBackoff{cfg: cfg.Backoff},
	}, nil
}

// Reconcile implements reconcile.Reconciler.
func (r *Reconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	item := NewObject[T]()
	if err := r.Client.Get(ctx, req.NamespacedName, item); err != nil {
		return r.backoff.forget(req.NamespacedName), client.IgnoreNotFound(err)
	}

	ctx, logger := log.NewContextLogger(ctx, req.NamespacedName.String(), "gen="+item.GetResourceVersion())
	ctx = utils.SetRequestID(ctx, fmt.Sprintf("%d", item.GetGeneration()))
	logger.Infof("Starting reconciliation")
	logger.Debugf("Item: %s", ToString(item))

	if item.GetDeletionTimestamp() != nil && !item.GetDeletionTimestamp().IsZero() {
		finalizers := item.GetFinalizers()
		if !slices.Contains(finalizers, r.cfg.FinalizerName) {
			logger.Infof("Object deleted")
			return r.backoff.forget(req.NamespacedName), nil
		}
		logger.Infof("Handling deletion")
		if err := r.deleteWf.Execute(ctx, item); err != nil {
			return r.Fail(ctx, item, err)
		}
		logger.Infof("Deletion handled successfully")
		slices.Remove(&finalizers, r.cfg.FinalizerName)
		item.SetFinalizers(finalizers)
		return r.backoff.forget(req.NamespacedName), r.Client.Update(ctx, item)
	}

	if !slices.Contains(item.GetFinalizers(), r.cfg.FinalizerName) {
		logger.Infof("Updating finalizers and requeueing")
		item.SetFinalizers(append(item.GetFinalizers(), r.cfg.FinalizerName))
		return r.backoff.forget(req.NamespacedName), r.Client.Update(ctx, item) // Will requeue
	}

	if err := r.applyWf.Execute(ctx, item); err != nil {
		return r.Fail(ctx, item, err)
	}
	logger.Infof("Reconciliation handled successfully")
	r.SetReady(item, metav1.ConditionTrue)
	return r.backoff.forget(req.NamespacedName), r.Client.Status().Update(ctx, item)
}

// Fail records err in the Ready condition and requeues the item.
func (r *Reconciler[T]) Fail(ctx context.Context, item T, err error) (ctrl.Result, error) {
	log.NewLoggerFromContext(ctx).Errorf(err, "failed to reconcile")
	r.SetReady(item, metav1.ConditionFalse, err.Error())
	if updateErr := r.Client.Status().Update(ctx, item); updateErr != nil {
		log.NewLoggerFromContext(ctx).Errorf(updateErr, "unable to update status")
	}
	return r.backoff.next(item), nil
}

// SetReady sets the Ready condition on item.
func (r *Reconciler[T]) SetReady(item T, status metav1.ConditionStatus, optReason ...string) {
	reason := opt.Opt("All services completed", optReason)
	meta.SetStatusCondition(item.GetConditions(), metav1.Condition{
		Type:               "Ready",
		Status:             status,
		ObservedGeneration: item.GetGeneration(),
		Reason:             reason,
	})
}

func StatusWrapper[T Obj](svc Service[T], action Action[T]) Step[T] {
	return func(ctx context.Context, item T) error {
		err := action(svc)(ctx, item)
		meta.SetStatusCondition(item.GetConditions(), metav1.Condition{
			Type:               svc.GetName(),
			Status:             StatusAdapter(err),
			ObservedGeneration: item.GetGeneration(),
			Reason:             ReasonAdapter(err),
		})
		return err
	}
}

func ToString[T Obj](item T) string {
	raw, _ := json.Marshal(item)
	return string(raw)
}

// NewObject returns a freshly allocated, non-nil instance of T. T is a pointer
// type (e.g. *MyCR) whose zero value is nil, so the pointed-to struct is
// allocated via reflection. This keeps the API to a single type parameter
// instead of the [T any, PT interface{ *T; Obj }] pattern; controller-runtime's
// reconcile.AsReconciler makes the same choice. See ADR-0004.
func NewObject[T Obj]() T {
	var zero T
	value := reflect.ValueOf(zero)
	if !value.IsValid() || value.Kind() != reflect.Pointer || !value.IsNil() {
		return zero
	}
	return reflect.New(value.Type().Elem()).Interface().(T)
}

func ReasonAdapter(err error) string {
	if err != nil {
		return err.Error() // TODO
	}
	return "Ready"
}

func StatusAdapter(err error) metav1.ConditionStatus {
	if err != nil {
		return metav1.ConditionFalse
	}
	return metav1.ConditionTrue
}
