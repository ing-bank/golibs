package runtime

import (
	"context"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/graceful"
	"github.com/ing-bank/golibs/pkg/kube/operator"
	"github.com/ing-bank/golibs/pkg/kubemock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

type AddToSchemeFunc func(*runtime.Scheme) error

// Config configures a Runtime for resource pointer type T, e.g.
// runtime.Config[*MyCR]{...}.
type Config[T operator.Obj] struct {
	UseFakeClient bool `json:"useFakeClient,omitempty"`

	Kube                     *rest.Config                                                     `json:"kube,omitempty"`
	Scheme                   *runtime.Scheme                                                  `json:"scheme,omitempty"`
	Schemes                  []AddToSchemeFunc                                                `json:"schemes,omitempty"`
	ControllerOpts           *ctrl.Options                                                    `json:"controllerOpts,omitempty"`
	ControllerManagerBuilder func(mgr ctrl.Manager, reconciler *operator.Reconciler[T]) error `json:"-"`

	Operator *operator.Config     `json:"operator,omitempty"`
	Services operator.Services[T] `json:"services,omitempty"`
}

// ApplyDefaults fills unset fields, including a ControllerManagerBuilder that
// watches T.
func (c *Config[T]) ApplyDefaults() {
	if c.Scheme == nil {
		c.Scheme = runtime.NewScheme()
	}
	if c.Operator == nil {
		c.Operator = &operator.Config{}
	}

	if c.ControllerOpts == nil {
		c.ControllerOpts = &ctrl.Options{}
	}

	if c.ControllerManagerBuilder == nil {
		c.ControllerManagerBuilder = func(mgr ctrl.Manager, reconciler *operator.Reconciler[T]) error {
			return ctrl.NewControllerManagedBy(mgr).For(operator.NewObject[T]()).Complete(reconciler)
		}
	}
}

// Prepare loads the in-cluster or kubeconfig rest config when none is set.
func (c *Config[T]) Prepare() error {
	if !c.UseFakeClient && c.Kube == nil {
		cfg, err := ctrl.GetConfig()
		if err != nil {
			return err
		}
		c.Kube = cfg
	}

	return nil
}

// Validate implements config validation; there are currently no constraints.
func (c *Config[T]) Validate() error {
	return nil
}

// Runtime bundles the operator reconciler wired to a controller manager.
type Runtime[T operator.Obj] struct {
	Operator          *operator.Reconciler[T]
	ControllerManager ctrl.Manager
}

// NewForConfig builds a controller manager and registers a reconciler for T.
// T is the resource pointer type, e.g. runtime.NewForConfig(runtime.Config[*MyCR]{...}).
func NewForConfig[T operator.Obj](cfg Config[T]) (*Runtime[T], error) {
	err := config.Configure(&cfg)
	if err != nil {
		return nil, err
	}

	// Populate schemes
	for _, addToScheme := range cfg.Schemes {
		if err := addToScheme(cfg.Scheme); err != nil {
			return nil, err
		}
	}
	cfg.ControllerOpts.Scheme = cfg.Scheme // Probably unnecessary

	var mgr ctrl.Manager
	if cfg.UseFakeClient {
		mgr, err = kubemock.NewManagerForFakeCluster(cfg.Kube, *cfg.ControllerOpts, kubemock.NewFakeCluster(cfg.Scheme))
	} else {
		mgr, err = ctrl.NewManager(cfg.Kube, *cfg.ControllerOpts)
	}
	if err != nil {
		return nil, err
	}

	reconciler := operator.NewReconciler[T](mgr.GetClient(), *cfg.Operator, cfg.Services...)
	if err := cfg.ControllerManagerBuilder(mgr, reconciler); err != nil {
		return nil, err
	}

	return &Runtime[T]{
		Operator:          reconciler,
		ControllerManager: mgr,
	}, nil
}

func (r *Runtime[T]) Start(ctx context.Context) error {
	return graceful.RunAll(ctx, r.ControllerManager.Start)
}
