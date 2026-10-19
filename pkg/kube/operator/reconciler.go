package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

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

	cfg Config
}

type Config struct {
	FinalizerName string `json:"finalizerName"`
}

func (c *Config) ApplyDefaults() {
	if c.FinalizerName == "" {
		c.FinalizerName = "finalizer"
	}
}

func NewReconciler[T Obj](client client.Client, cfg Config, services ...Service[T]) *Reconciler[T] {
	cfg.ApplyDefaults()
	return &Reconciler[T]{
		Client:   client,
		applyWf:  Services[T](services).asWorkflow("apply", ActionApply[T], StatusWrapper[T]),
		deleteWf: Services[T](services).asWorkflow("delete", ActionDelete[T], StatusWrapper[T]),
		cfg:      cfg,
	}
}

func (r *Reconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	item := zeroValue[T]()
	if err := r.Client.Get(ctx, req.NamespacedName, item); err != nil {
		return ctrl.Result{}, nil
	}

	ctx, logger := log.NewContextLogger(ctx, req.NamespacedName.String(), "gen="+item.GetResourceVersion())
	ctx = utils.SetRequestID(ctx, fmt.Sprintf("%d", item.GetGeneration()))
	logger.Infof("Starting reconciliation")
	logger.Debugf("Item: %s", ToString(item))

	if item.GetDeletionTimestamp() != nil && !item.GetDeletionTimestamp().IsZero() {
		finalizers := item.GetFinalizers()
		if !slices.Contains(finalizers, r.cfg.FinalizerName) {
			logger.Infof("Object deleted")
			return ctrl.Result{}, nil
		}
		logger.Infof("Handling deletion")
		if err := r.deleteWf.Execute(ctx, item); err != nil {
			return r.Fail(ctx, item, err)
		}
		logger.Infof("Deletion handled successfully")
		slices.Remove(&finalizers, r.cfg.FinalizerName)
		item.SetFinalizers(finalizers)
		return ctrl.Result{}, r.Client.Update(ctx, item)
	}

	if !slices.Contains(item.GetFinalizers(), r.cfg.FinalizerName) {
		logger.Infof("Updating finalizers and requeueing")
		item.SetFinalizers(append(item.GetFinalizers(), r.cfg.FinalizerName))
		return ctrl.Result{}, r.Client.Update(ctx, item) // Will requeue
	}

	if err := r.applyWf.Execute(ctx, item); err != nil {
		return r.Fail(ctx, item, err)
	}
	logger.Infof("Reconciliation handled successfully")
	r.SetReady(item, metav1.ConditionTrue)
	return ctrl.Result{}, r.Client.Status().Update(ctx, item)
}

func (r *Reconciler[T]) Fail(ctx context.Context, item T, err error) (ctrl.Result, error) {
	log.NewLoggerFromContext(ctx).Errorf(err, "failed to reconcile")
	r.SetReady(item, metav1.ConditionFalse, err.Error())
	if updateErr := r.Client.Status().Update(ctx, item); updateErr != nil {
		log.NewLoggerFromContext(ctx).Errorf(updateErr, "unable to update status")
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil // TODO: exponential backoff
}

func (r *Reconciler[T]) SetReady(item T, status metav1.ConditionStatus, optReason ...string) {
	reason := opt.Opt("Ready", optReason)
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

func zeroValue[T Obj]() T {
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
