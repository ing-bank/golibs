package configmap

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type testValue struct {
	Name string
	Bar  int
}

func (t testValue) GetName() string {
	return t.Name
}

// newTestStore creates a ConfigMap store for tests.
// immutableLabels: labels that cannot be overridden.
// enricher: function to enrich labels based on value.
func newTestStore(t *testing.T, immutableLabels map[string]string, enricher func(testValue) (map[string]string, error)) *ConfigMap[testValue] {
	cfg := Config[testValue]{
		Namespace:       "default",
		ImmutableLabels: immutableLabels,
		LabelsEnricher:  enricher,
	}
	db, err := NewFake[testValue](cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return db.(*ConfigMap[testValue])
}

func TestCreateReadUpdateDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, nil, nil)
	val := testValue{Name: "alpha", Bar: 42}

	if err := store.Create(ctx, val.GetName(), val); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := store.Read(ctx, val.GetName())
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got != val {
		t.Errorf("Read value mismatch: got %+v, want %+v", got, val)
	}

	val.Bar = 100
	if err := store.Update(ctx, val.GetName(), val); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, err = store.Read(ctx, val.GetName())
	if err != nil {
		t.Fatalf("Read after update failed: %v", err)
	}
	if got != val {
		t.Errorf("Update value mismatch: got %+v, want %+v", got, val)
	}

	if err := store.Delete(ctx, val.GetName()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = store.Read(ctx, val.GetName())
	if err == nil {
		t.Errorf("expected error after delete, got nil")
	}
}

func TestApply(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, nil, nil)
	val := testValue{Name: "gamma", Bar: 1}

	if err := store.Apply(ctx, val.GetName(), val); err != nil {
		t.Fatalf("Apply (create) failed: %v", err)
	}
	got, err := store.Read(ctx, val.GetName())
	if err != nil || got != val {
		t.Errorf("Apply (create) mismatch: got %+v, want %+v, err=%v", got, val, err)
	}

	val2 := testValue{Name: "delta", Bar: 2}
	if err := store.Apply(ctx, val2.GetName(), val2); err != nil {
		t.Fatalf("Apply (update) failed: %v", err)
	}
	got, err = store.Read(ctx, val2.GetName())
	if err != nil || got != val2 {
		t.Errorf("Apply (update) mismatch: got %+v, want %+v, err=%v", got, val2, err)
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, nil, nil)
	values := []testValue{
		{Name: "epsilon", Bar: 1},
		{Name: "zeta", Bar: 2},
		{Name: "eta", Bar: 3},
	}
	for _, v := range values {
		if err := store.Create(ctx, v.GetName(), v); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}
	items, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(items) != len(values) {
		t.Errorf("List count mismatch: got %d, want %d", len(items), len(values))
	}
	names := map[string]bool{"epsilon": false, "zeta": false, "eta": false}
	for _, item := range items {
		if _, ok := names[item.Key]; ok {
			names[item.Key] = true
		}
	}
	for k, found := range names {
		if !found {
			t.Errorf("List did not return item with name %s", k)
		}
	}
}

func TestCreateWithLabels(t *testing.T) {
	ctx := context.Background()
	db := newTestStore(t, nil, nil)
	val := testValue{Name: "theta", Bar: 10}
	lbls := labelstore.Labels{"custom": "customval", "team": "dev"}

	if err := db.Create(ctx, val.GetName(), val, labelstore.WithLabels(lbls)); err != nil {
		t.Fatalf("Create with labels failed: %v", err)
	}
	cm, err := db.client.Get(ctx, val.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get configmap failed: %v", err)
	}
	for k, v := range lbls {
		if cm.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %s", k, v, cm.Labels[k])
		}
	}
}

func TestCreateInvalidJSON(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, nil, nil)
	val := testValue{Name: "iota", Bar: 0}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: val.GetName()},
		Data:       map[string]string{"payload": "not-json"},
	}
	_, err := store.client.Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create configmap: %v", err)
	}
	_, err = store.Read(ctx, val.GetName())
	if err == nil {
		t.Errorf("expected error for invalid JSON, got nil")
	}
}

func TestCreateWithLabelsEnricher(t *testing.T) {
	ctx := context.Background()
	// Enricher sets enriched label
	db := newTestStore(t, nil, func(val testValue) (map[string]string, error) {
		return map[string]string{"enriched-name": val.Name, "enriched-bar": fmt.Sprintf("%d", val.Bar)}, nil
	})
	val := testValue{Name: "kappa", Bar: 123}

	if err := db.Create(ctx, val.GetName(), val); err != nil {
		t.Fatalf("Create with LabelsEnricher failed: %v", err)
	}
	cm, err := db.client.Get(ctx, val.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get configmap failed: %v", err)
	}
	if cm.Labels["enriched-name"] != val.Name {
		t.Errorf("expected enriched-name=%s, got %s", val.Name, cm.Labels["enriched-name"])
	}
	if cm.Labels["enriched-bar"] != "123" {
		t.Errorf("expected enriched-bar=123, got %s", cm.Labels["enriched-bar"])
	}
}

func TestListWithLabelSelector(t *testing.T) {
	ctx := context.Background()
	// Enricher sets enriched label
	db := newTestStore(t, nil, func(val testValue) (map[string]string, error) {
		return map[string]string{"enriched-bar": fmt.Sprintf("%d", val.Bar)}, nil
	})
	entries := []testValue{
		{Name: "Dev-One", Bar: 1},   // 1=Dev
		{Name: "Dev-Two", Bar: 1},   // 1=Dev
		{Name: "Acc-Three", Bar: 2}, // 2=Acc
		{Name: "Prod-Four", Bar: 3}, // 3=Prod
	}
	for _, v := range entries {
		if err := db.Create(ctx, v.GetName(), v); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}
	selector := "enriched-bar=1"
	items, err := db.List(ctx, labelstore.WithLabelSelector(selector))
	if err != nil {
		t.Fatalf("List with label selector failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items for enriched=dev, got %d", len(items))
	}
	for _, item := range items {
		if !strings.HasPrefix(item.Value.Name, "Dev") {
			t.Errorf("expected Name=dev, got %s", item.Value.Name)
		}
	}
	selector = "enriched-bar=3"
	items, err = db.List(ctx, labelstore.WithLabelSelector(selector))
	if err != nil {
		t.Fatalf("List with label selector failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item for enriched=prod, got %d", len(items))
	}
	for _, item := range items {
		if !strings.HasPrefix(item.Value.Name, "Prod") {
			t.Errorf("expected Name=prod, got %s", item.Value.Name)
		}
	}
	selector = "enriched-bar=2"
	items, err = db.List(ctx, labelstore.WithLabelSelector(selector))
	if err != nil {
		t.Fatalf("List with label selector failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 items for enriched=qa, got %d", len(items))
	}
}

func TestLabelOverridePrecedence(t *testing.T) {
	ctx := context.Background()
	// immutableLabels cannot be overridden, enriched labels can be overridden by custom labels
	immutableLabels := map[string]string{"immutable": "always"}
	enricher := func(val testValue) (map[string]string, error) {
		return map[string]string{"enriched": "enrichval", "env": "enrichenv"}, nil
	}
	db := newTestStore(t, immutableLabels, enricher)
	val := testValue{Name: "enrich", Bar: 1}
	customLabels := labelstore.Labels{"enriched": "customval", "bar": "custombar"}

	// Should succeed: immutable labels present, enriched overridden by custom
	if err := db.Create(ctx, val.GetName(), val, labelstore.WithLabels(customLabels)); err != nil {
		t.Fatalf("Create with label override failed: %v", err)
	}
	cm, err := db.client.Get(ctx, val.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get configmap failed: %v", err)
	}
	// Immutable labels always present
	if cm.Labels["immutable"] != "always" {
		t.Errorf("expected immutable=always, got %s", cm.Labels["immutable"])
	}
	if cm.Labels["env"] != "enrichenv" {
		t.Errorf("expected env=enrichenv, got %s", cm.Labels["env"])
	}
	// Custom label overrides enricher
	if cm.Labels["enriched"] != "customval" {
		t.Errorf("expected enriched=customval, got %s", cm.Labels["enriched"])
	}
	// Custom label present
	if cm.Labels["bar"] != "custombar" {
		t.Errorf("expected bar=custombar, got %s", cm.Labels["bar"])
	}
}

func TestImmutableLabelCannotBeOverriddenByCustom(t *testing.T) {
	ctx := context.Background()
	immutableLabels := map[string]string{"immutable": "always"}
	store := newTestStore(t, immutableLabels, nil)

	val := testValue{Name: "fail", Bar: 2}
	customLabels := labelstore.Labels{"immutable": "fail", "env": "fail"}

	err := store.Create(ctx, val.GetName(), val, labelstore.WithLabels(customLabels))
	if err == nil || !strings.Contains(err.Error(), "label immutable is immutable") {
		t.Errorf("expected error on immutable label override, got: %v", err)
	}
}

func TestImmutableLabelCannotBeOverriddenByEnricher(t *testing.T) {
	ctx := context.Background()
	immutableLabels := map[string]string{"immutable": "always", "env": "prod"}
	enricher := func(val testValue) (map[string]string, error) {
		return map[string]string{"immutable": "fail"}, nil
	}
	db := newTestStore(t, immutableLabels, enricher)

	val := testValue{Name: "fail", Bar: 2}

	err := db.Create(ctx, val.GetName(), val)
	if err == nil || !strings.Contains(err.Error(), "label immutable is immutable") {
		t.Errorf("expected error on immutable label override by enricher, got: %v", err)
	}
}

func TestListWithImmutableLabelSelector(t *testing.T) {
	ctx := context.Background()
	immutableLabels := map[string]string{"immutable": "always", "env": "prod"}
	store := newTestStore(t, immutableLabels, nil)

	val := testValue{Name: "foo", Bar: 1}
	if err := store.Create(ctx, val.GetName(), val); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	// Selector tries to override immutable label
	selector := "immutable=other"
	_, err := store.List(ctx, labelstore.WithLabelSelector(selector))
	if err == nil || !strings.Contains(err.Error(), "cannot override") {
		t.Errorf("expected error when overriding immutable label selector, got: %v", err)
	}
}

func TestWithPrefix(t *testing.T) {
	ctx := context.Background()
	immutableLabels := map[string]string{"immutable": "always"}
	cfg := Config[testValue]{
		Namespace:       "default",
		ImmutableLabels: immutableLabels,
	}
	db, err := NewFake[testValue](cfg)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	val1 := testValue{Name: "pre-1", Bar: 123}
	if err := db.Create(ctx, val1.GetName(), val1); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	val2 := testValue{Name: "pre-2", Bar: 123}
	if err := db.Create(ctx, val2.GetName(), val2); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Match correct prefix
	items, err := db.List(ctx, store.WithPrefix("pre-"))
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with prefix 'pre-', got %d", len(items))
	}
	for _, item := range items {
		if !strings.HasPrefix(item.Key, "pre-") {
			t.Errorf("expected key with prefix 'pre-', got %s", item.Key)
		}
	}

	// Match non existing prefix
	items, err = db.List(ctx, store.WithPrefix("nonexistent-"))
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items with prefix 'nonexistent-', got %d", len(items))
	}
}
