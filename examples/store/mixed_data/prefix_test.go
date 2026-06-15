package mixed_data

import (
	"context"
	"fmt"
	"log"

	"github.com/ing-bank/golibs/pkg/kubemock"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/configmap"
	"github.com/ing-bank/golibs/pkg/store/middleware/prefix"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
	"k8s.io/client-go/kubernetes/fake"
)

type Foo struct {
	Name string `json:"name"`
}

type Bar struct {
	Age int `json:"age"`
}

func Example() {
	ctx := context.Background()

	// --- Setup ---

	// Create a shared backend and two stores with different types and prefixes.
	// ONE store holds all data. In this case the mocked Kube API holds ConfigMaps with both Foo and Bar data.
	kube := fake.NewSimpleClientset()
	lister := kubemock.NewMockConfigMapLister(kube)
	fooStore, err := store.New(
		configmap.NewBackend(configmap.Config[*Foo]{Namespace: "example-dev"}, lister, kube.CoreV1().ConfigMaps("example-dev")),
		threadsafe.New,
		prefix.NewBuilder[*Foo]("foo-"), // Prefix all data with foo-
	)
	if err != nil {
		log.Fatalf("failed to create foo store: %v", err)
	}

	barStore, err := store.New(
		configmap.NewBackend(configmap.Config[*Bar]{Namespace: "example-dev"}, lister, kube.CoreV1().ConfigMaps("example-dev")),
		threadsafe.New,
		prefix.NewBuilder[*Bar]("bar-"), // Prefix all data with bar- (different from foo store, so no conflicts)
	)
	if err != nil {
		log.Fatalf("failed to create bar store: %v", err)
	}

	// --- Execution ---

	err = fooStore.Create(ctx, "1", &Foo{Name: "foo"})
	if err != nil {
		log.Fatalf("failed to create foo: %v", err)
	}

	// Note that we can use the same backend for both stores, even with different types, because the prefix middleware
	// ensures that keys don't conflict.
	err = barStore.Create(ctx, "1", &Bar{Age: 42})
	if err != nil {
		log.Fatalf("failed to apply bar: %v", err)
	}

	foo, err := fooStore.Read(ctx, "1")
	if err != nil {
		log.Fatalf("failed to read foo: %v", err)
	}
	fmt.Println(foo.Name)

	bar, err := barStore.Read(ctx, "1")
	if err != nil {
		log.Fatalf("failed to read bar: %v", err)
	}
	fmt.Println(bar.Age)

	// TODO: write example test in prefix package

	// Output:
	// foo
	// 42
}
