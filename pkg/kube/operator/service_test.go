package operator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/kubemock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

type testStatus struct {
	Ready      bool
	Message    string
	Conditions []metav1.Condition
}

func (s testStatus) clone() testStatus {
	out := s
	if s.Conditions != nil {
		out.Conditions = append([]metav1.Condition(nil), s.Conditions...)
	}
	return out
}

type testObj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            testStatus `json:"status,omitempty"`
}

func (o *testObj) DeepCopyObject() runtime.Object {
	if o == nil {
		return nil
	}
	out := *o
	out.ObjectMeta = *o.ObjectMeta.DeepCopy()
	out.Status = o.Status.clone()
	return &out
}

func (o *testObj) GetConditions() *[]metav1.Condition { return &o.Status.Conditions }

type testObjList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []testObj `json:"items"`
}

func (l *testObjList) DeepCopyObject() runtime.Object {
	if l == nil {
		return nil
	}
	out := *l
	if l.Items != nil {
		out.Items = make([]testObj, len(l.Items))
		for i := range l.Items {
			out.Items[i] = *l.Items[i].DeepCopyObject().(*testObj)
		}
	}
	return &out
}

type testService struct {
	name      string
	applyErr  error
	deleteErr error
	calls     *[]string
}

func (s *testService) GetName() string { return s.name }

func (s *testService) Apply(ctx context.Context, obj *testObj) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, s.name)
	}
	obj.Status.Ready = true
	obj.Status.Message = s.name
	return s.applyErr
}

func (s *testService) Delete(ctx context.Context, obj *testObj) error {
	if s.calls != nil {
		*s.calls = append(*s.calls, "delete:"+s.name)
	}
	obj.Status.Ready = false
	obj.Status.Message = "deleted-" + s.name
	return s.deleteErr
}

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
	scheme.AddKnownTypeWithName(gv.WithKind("Test"), &testObj{})
	scheme.AddKnownTypeWithName(gv.WithKind("TestList"), &testObjList{})
	return scheme
}

func TestActionApplyAndDelete(t *testing.T) {
	obj := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
	applySvc := &testService{name: "apply"}
	deleteSvc := &testService{name: "delete"}

	if err := ActionApply[*testObj](applySvc)(context.Background(), obj); err != nil {
		t.Fatalf("apply action returned error: %v", err)
	}
	if !obj.Status.Ready || obj.Status.Message != "apply" {
		t.Fatalf("apply action did not set expected status: %+v", obj.Status)
	}
	if err := ActionDelete[*testObj](deleteSvc)(context.Background(), obj); err != nil {
		t.Fatalf("delete action returned error: %v", err)
	}
	if obj.Status.Ready || obj.Status.Message != "deleted-delete" {
		t.Fatalf("delete action did not clear status as expected: %+v", obj.Status)
	}
}

func TestServicesWorkflowRunsAllServicesInOrder(t *testing.T) {
	calls := make([]string, 0, 2)
	svc1 := &testService{name: "first", calls: &calls}
	svc2 := &testService{name: "second", calls: &calls}

	wf := Services[*testObj]{svc1, svc2}.asWorkflow("apply", ActionApply[*testObj], StatusWrapper[*testObj])
	obj := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}

	if err := wf.Execute(context.Background(), obj); err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if len(calls) != 2 || calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("workflow executed in unexpected order: %#v", calls)
	}
	if obj.Status.Message != "second" {
		t.Fatalf("last service did not set final message: %+v", obj.Status)
	}
}

func TestStatusWrapperSetsConditionOnSuccessAndFailure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &testService{name: "ready-check"}
		obj := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}

		if err := StatusWrapper[*testObj](svc, ActionApply[*testObj])(context.Background(), obj); err != nil {
			t.Fatalf("status wrapper returned error on success path: %v", err)
		}
		if len(obj.Status.Conditions) != 1 {
			t.Fatalf("expected one status condition, got %d", len(obj.Status.Conditions))
		}
		cond := obj.Status.Conditions[0]
		if cond.Type != "ready-check" || cond.Status != metav1.ConditionTrue || cond.Reason != "Ready" {
			t.Fatalf("unexpected condition: %#v", cond)
		}
	})

	t.Run("failure", func(t *testing.T) {
		svc := &testService{name: "broken", applyErr: errors.New("boom")}
		obj := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}

		err := StatusWrapper[*testObj](svc, ActionApply[*testObj])(context.Background(), obj)
		if err == nil || err.Error() != "boom" {
			t.Fatalf("expected boom error, got %v", err)
		}
		cond := obj.Status.Conditions[0]
		if cond.Type != "broken" || cond.Status != metav1.ConditionFalse || cond.Reason != "boom" {
			t.Fatalf("unexpected failure condition: %#v", cond)
		}
	})
}

func TestReconcilerReconcileUpdatesStatusAndAddsFinalizer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scheme := newTestScheme()
	shared := kubemock.NewSharedFake(scheme)
	client := shared.Client()

	obj := &testObj{
		TypeMeta: metav1.TypeMeta{APIVersion: "test.example.com/v1", Kind: "Test"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "demo",
			Namespace:  "default",
			Finalizers: []string{"finalizer"},
		},
	}
	if err := client.Create(ctx, obj); err != nil {
		t.Fatalf("failed to create test object: %v", err)
	}

	reconciler := NewReconciler[*testObj](client, Config{FinalizerName: "finalizer"}, &testService{name: "reconcile-step"})
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "demo", Namespace: "default"}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Fatalf("unexpected reconcile result: %#v", result)
	}

	stored := &testObj{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo", Namespace: "default"}, stored); err != nil {
		t.Fatalf("unable to read reconciled object: %v", err)
	}
	if len(stored.Status.Conditions) != 2 {
		t.Fatalf("expected two conditions (service + ready), got %d: %#v", len(stored.Status.Conditions), stored.Status.Conditions)
	}
	// Check service condition
	if stored.Status.Conditions[0].Type != "reconcile-step" {
		t.Fatalf("expected first condition to be reconcile-step, got %s", stored.Status.Conditions[0].Type)
	}
	// Check ready condition
	if stored.Status.Conditions[1].Type != "Ready" || stored.Status.Conditions[1].Status != metav1.ConditionTrue {
		t.Fatalf("expected Ready condition to be True, got %#v", stored.Status.Conditions[1])
	}
}

func TestReconcilerFailSetsReadyFalseAndRequeues(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scheme := newTestScheme()
	shared := kubemock.NewSharedFake(scheme)
	client := shared.Client()

	obj := &testObj{
		TypeMeta: metav1.TypeMeta{APIVersion: "test.example.com/v1", Kind: "Test"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "demo",
			Namespace:  "default",
			Finalizers: []string{"finalizer"},
		},
	}
	if err := client.Create(ctx, obj); err != nil {
		t.Fatalf("failed to create failing object: %v", err)
	}

	reconciler := NewReconciler[*testObj](client, Config{FinalizerName: "finalizer"}, &testService{name: "fail-step", applyErr: errors.New("boom")})
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "demo", Namespace: "default"}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != 10_000_000_000 {
		t.Fatalf("unexpected requeue duration: %#v", result)
	}

	stored := &testObj{}
	if err := client.Get(ctx, types.NamespacedName{Name: "demo", Namespace: "default"}, stored); err != nil {
		t.Fatalf("unable to read failing object: %v", err)
	}
	// Check that we have at least 2 conditions: service condition and Ready condition
	if len(stored.Status.Conditions) < 2 {
		t.Fatalf("expected at least two conditions, got %d: %#v", len(stored.Status.Conditions), stored.Status.Conditions)
	}
	// Find the Ready condition
	var readyCondition *metav1.Condition
	for i, cond := range stored.Status.Conditions {
		if cond.Type == "Ready" {
			readyCondition = &stored.Status.Conditions[i]
			break
		}
	}
	if readyCondition == nil || readyCondition.Status != metav1.ConditionFalse {
		t.Fatalf("expected Ready condition to be False, got %#v", readyCondition)
	}
}

