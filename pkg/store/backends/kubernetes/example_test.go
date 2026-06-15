package kubernetes

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Example() {
	db := NewFake[*v1.ConfigMap](Config{Group: "", Version: "v1", Resource: "configmaps"})

	ctx := context.Background()
	err := db.Apply(ctx, "1", &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "1",
		},
	}, store.DryRun)
	if err != nil {
		panic(err)
	}
	items, _ := db.List(ctx, store.ListKeysOnly)
	fmt.Println(items)

	// Output:
	// []
}
