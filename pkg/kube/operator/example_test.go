package operator

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ExampleStatusWrapper() {
	obj := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	_ = StatusWrapper[*testObj](&testService{name: "demo-step"}, ActionApply[*testObj])(context.Background(), obj)
	fmt.Println(obj.Status.Conditions[0].Type, obj.Status.Conditions[0].Status)
	// Output: demo-step True
}
