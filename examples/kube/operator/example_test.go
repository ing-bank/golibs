// This example shows the layout of an operator built on pkg/kube/runtime:
//
//   - api/v1: the custom resource (model) and its scheme registration
//   - service: the operator logic as operator.Service implementations
//   - this file: the setup, which in a real project lives in main.go
//
// Services do not touch status. The reconciler records one condition per
// service (named after GetName) and sets "Ready" once all services succeeded.
//
// UseFakeClient runs the operator on an in-memory kubemock cluster. Drop it
// (and set Config.Kube) to run against a real cluster.
//
// Logger is installed as controller-runtime's global logger. It defaults to
// the logrus standard logger (the same backend pkg/task/workflow uses), so it
// is only set here to show where to plug in your own logrus or logr logger.
package main

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/ing-bank/golibs/examples/kube/operator/api/v1"
	"github.com/ing-bank/golibs/examples/kube/operator/service"
	kubelog "github.com/ing-bank/golibs/pkg/kube/log"
	"github.com/ing-bank/golibs/pkg/kube/operator"
	"github.com/ing-bank/golibs/pkg/kube/runtime"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Example() {
	runner, err := runtime.NewForConfig(runtime.Config[*v1.Widget]{
		UseFakeClient: true,
		Schemes:       []runtime.AddToSchemeFunc{v1.AddToScheme},
		Services:      operator.Services[*v1.Widget]{service.NewProvision()},
		Logger:        kubelog.NewLogrusLogger(logrus.StandardLogger()),
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Act as a user of the cluster: create a Widget and wait for the operator.
	go func() {
		defer cancel() // stop the operator once the status has been observed
		c := runner.ControllerManager.GetClient()
		if !runner.ControllerManager.GetCache().WaitForCacheSync(ctx) {
			return
		}

		w := &v1.Widget{}
		w.Name, w.Namespace = "demo", "default"
		if err := c.Create(ctx, w); err != nil {
			panic(err)
		}
		fmt.Println("created:", conditions(w))

		key := client.ObjectKeyFromObject(w)
		for ctx.Err() == nil {
			if err := c.Get(ctx, key, w); err == nil && meta.IsStatusConditionTrue(w.Conditions.Conditions, "Ready") {
				fmt.Println("reconciled:", conditions(w))
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	if err := runner.Start(ctx); err != nil {
		panic(err)
	}

	// Output:
	// created: []
	// reconciled: [Provisioned=True Ready=True]
}

// conditions renders the Widget's status conditions as Type=Status pairs.
func conditions(w *v1.Widget) []string {
	out := []string{}
	for _, c := range w.Conditions.Conditions {
		out = append(out, c.Type+"="+string(c.Status))
	}
	return out
}
