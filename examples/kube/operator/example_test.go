// This example shows the layout of an operator built on pkg/kube/runtime:
//
//   - api/v1: the custom resource (model) and its scheme registration
//   - service: the operator logic as operator.Service implementations
//   - this file: the setup, which in a real project lives in main.go
//
// UseFakeClient runs the operator on an in-memory kubemock cluster. Drop it
// (and set Config.Kube) to run against a real cluster.
package main

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/ing-bank/golibs/examples/kube/operator/api/v1"
	"github.com/ing-bank/golibs/examples/kube/operator/service"
	"github.com/ing-bank/golibs/pkg/kube/operator"
	"github.com/ing-bank/golibs/pkg/kube/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Example() {
	runner, err := runtime.NewForConfig(runtime.Config[*v1.Widget]{
		UseFakeClient: true,
		Schemes:       []runtime.AddToSchemeFunc{v1.AddToScheme},
		Services:      operator.Services[*v1.Widget]{service.NewReady()},
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
		fmt.Println("created, ready:", service.ReadyStatus(w))

		key := client.ObjectKeyFromObject(w)
		for ctx.Err() == nil {
			if err := c.Get(ctx, key, w); err == nil && service.ReadyStatus(w) != "<unset>" {
				fmt.Println("reconciled, ready:", service.ReadyStatus(w))
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	if err := runner.Start(ctx); err != nil {
		panic(err)
	}

	// Output:
	// created, ready: <unset>
	// reconciled, ready: True
}
