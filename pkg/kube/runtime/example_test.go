package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/kube/operator"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WidgetGroupVersion is the API group/version Widget is registered under.
var WidgetGroupVersion = schema.GroupVersion{Group: "example.golibs.ing.com", Version: "v1"}

// AddWidgetToScheme registers Widget and WidgetList; pass it via Config.Schemes.
func AddWidgetToScheme(s *kruntime.Scheme) error {
	s.AddKnownTypes(WidgetGroupVersion, &Widget{}, &WidgetList{})
	metav1.AddToGroupVersion(s, WidgetGroupVersion)
	return nil
}

// Widget is a minimal custom resource satisfying operator.Obj.
type Widget struct {
	metav1.TypeMeta     `json:",inline"`
	metav1.ObjectMeta   `json:"metadata"`
	operator.Conditions `json:"status"`
}

func (w *Widget) DeepCopyObject() kruntime.Object {
	out := *w
	w.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Conditions.Conditions = append([]metav1.Condition(nil), w.Conditions.Conditions...)
	return &out
}

// WidgetList is required by informers to list and watch Widgets.
type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Widget `json:"items"`
}

func (l *WidgetList) DeepCopyObject() kruntime.Object {
	out := *l
	l.ListMeta.DeepCopyInto(&out.ListMeta)
	out.Items = make([]Widget, len(l.Items))
	for i := range l.Items {
		out.Items[i] = *l.Items[i].DeepCopyObject().(*Widget)
	}
	return &out
}

var _ operator.Obj = (*Widget)(nil)

// NewReadyService returns a dummy operator.Service that only marks a Widget
// as ready. Real services would create or update dependent resources here.
func NewReadyService() operator.Service[*Widget] {
	svc, _ := operator.NewAnonService[*Widget]("ReadyService",
		operator.WithApply(func(ctx context.Context, w *Widget) error {
			meta.SetStatusCondition(w.GetConditions(), metav1.Condition{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "WidgetReady",
				ObservedGeneration: w.GetGeneration(),
			})
			return nil
		}),
		operator.WithDelete(func(ctx context.Context, w *Widget) error { return nil }),
	)
	return svc
}

func readyStatus(w *Widget) string {
	if c := meta.FindStatusCondition(w.Conditions.Conditions, "Ready"); c != nil {
		return string(c.Status)
	}
	return "<unset>"
}

func Example() {
	runner, err := NewForConfig(Config[*Widget]{
		UseFakeClient: true,
		Schemes:       []AddToSchemeFunc{AddWidgetToScheme},
		Services:      operator.Services[*Widget]{NewReadyService()},
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		defer cancel() // stop the runtime once the status has been observed
		c := runner.ControllerManager.GetClient()
		if !runner.ControllerManager.GetCache().WaitForCacheSync(ctx) {
			return
		}

		w := &Widget{Name: "demo", Namespace: "default"}
		if err := c.Create(ctx, w); err != nil {
			panic(err)
		}
		fmt.Println("created, ready:", readyStatus(w))

		key := client.ObjectKeyFromObject(w)
		for ctx.Err() == nil {
			if err := c.Get(ctx, key, w); err == nil && readyStatus(w) != "<unset>" {
				fmt.Println("reconciled, ready:", readyStatus(w))
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
