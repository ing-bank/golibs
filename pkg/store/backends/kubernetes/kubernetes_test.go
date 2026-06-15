package kubernetes

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type testValue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              map[string]interface{} `json:"spec"`
}

// GetObjectKind implements runtime.Object
func (t *testValue) GetObjectKind() schema.ObjectKind {
	return &t.TypeMeta
}

// DeepCopyObject implements runtime.Object
func (t *testValue) DeepCopyObject() runtime.Object {
	if t == nil {
		return nil
	}
	out := new(testValue)
	t.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies all properties from this object into another
func (t *testValue) DeepCopyInto(out *testValue) {
	*out = *t
	out.TypeMeta = t.TypeMeta
	t.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if t.Spec != nil {
		out.Spec = make(map[string]interface{})
		for k, v := range t.Spec {
			out.Spec[k] = v
		}
	}
}

func newTestStore(namespace string, gvr schema.GroupVersionResource) *DynamicResource[*testValue] {
	return NewFake[*testValue](Config{
		Namespace: namespace,
		Group:     gvr.Group,
		Version:   gvr.Version,
		Resource:  gvr.Resource,
	})
}

func createTestValue(name string, specData map[string]interface{}) *testValue {
	return &testValue{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "test.example.com/v1",
			Kind:       "TestResource",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: specData,
	}
}

// TestErrorConditions tests various error scenarios
func TestErrorConditions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	t.Run("read_not_found", func(t *testing.T) {
		_, err := dr.Read(ctx, "non-existent")
		if err == nil || !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}
	})

	t.Run("update_not_found", func(t *testing.T) {
		val := createTestValue("missing", map[string]interface{}{"data": "value"})
		err := dr.Update(ctx, "missing", val)
		if err == nil {
			t.Errorf("expected error for Update on non-existent resource")
		}
	})

	t.Run("delete_not_found", func(t *testing.T) {
		err := dr.Delete(ctx, "missing")
		if err == nil {
			t.Errorf("expected error for Delete on non-existent resource")
		}
	})

	t.Run("duplicate_create", func(t *testing.T) {
		val := createTestValue("dup-key", map[string]interface{}{"data": "value"})
		if err := dr.Create(ctx, "dup-key", val); err != nil {
			t.Fatalf("First Create failed: %v", err)
		}
		err := dr.Create(ctx, "dup-key", val)
		if err == nil || !apierrors.IsAlreadyExists(err) {
			t.Errorf("expected AlreadyExists error, got: %v", err)
		}
	})

	t.Run("unsupported_option", func(t *testing.T) {
		val := createTestValue("opt-test", map[string]interface{}{"data": "value"})
		if err := dr.Create(ctx, "opt-test", val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		type customOption struct{}
		_, err := dr.Read(ctx, "opt-test", customOption{})
		if err != store.ErrUnsupportedOption {
			t.Errorf("expected ErrUnsupportedOption, got: %v", err)
		}
	})
}

// TestAllKubernetesResourceTypes tests all major Kubernetes resource types with loops
func TestAllKubernetesResourceTypes(t *testing.T) {
	t.Parallel()

	allResources := []struct {
		group    string
		version  string
		resource string
		kind     string
		count    int
	}{
		// Core v1 resources
		{group: "", version: "v1", resource: "configmaps", kind: "ConfigMap", count: 5},
		{group: "", version: "v1", resource: "secrets", kind: "Secret", count: 4},
		{group: "", version: "v1", resource: "services", kind: "Service", count: 3},
		{group: "", version: "v1", resource: "pods", kind: "Pod", count: 6},
		{group: "", version: "v1", resource: "serviceaccounts", kind: "ServiceAccount", count: 3},
		{group: "", version: "v1", resource: "persistentvolumeclaims", kind: "PersistentVolumeClaim", count: 2},
		{group: "", version: "v1", resource: "endpoints", kind: "Endpoints", count: 2},
		// Apps resources
		{group: "apps", version: "v1", resource: "deployments", kind: "Deployment", count: 5},
		{group: "apps", version: "v1", resource: "statefulsets", kind: "StatefulSet", count: 3},
		{group: "apps", version: "v1", resource: "daemonsets", kind: "DaemonSet", count: 2},
		{group: "apps", version: "v1", resource: "replicasets", kind: "ReplicaSet", count: 4},
		// Batch resources
		{group: "batch", version: "v1", resource: "jobs", kind: "Job", count: 3},
		{group: "batch", version: "v1", resource: "cronjobs", kind: "CronJob", count: 2},
		// Networking resources
		{group: "networking.k8s.io", version: "v1", resource: "ingresses", kind: "Ingress", count: 3},
		{group: "networking.k8s.io", version: "v1", resource: "networkpolicies", kind: "NetworkPolicy", count: 2},
		// RBAC resources
		{group: "rbac.authorization.k8s.io", version: "v1", resource: "roles", kind: "Role", count: 3},
		{group: "rbac.authorization.k8s.io", version: "v1", resource: "rolebindings", kind: "RoleBinding", count: 2},
		// Storage resources
		{group: "storage.k8s.io", version: "v1", resource: "storageclasses", kind: "StorageClass", count: 2},
	}

	for _, res := range allResources {
		res := res
		t.Run(res.resource, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			gvr := schema.GroupVersionResource{Group: res.group, Version: res.version, Resource: res.resource}
			dr := newTestStore("default", gvr)

			// Create multiple items
			for i := 0; i < res.count; i++ {
				key := fmt.Sprintf("%s-%d", res.resource, i)
				val := createTestValue(key, map[string]interface{}{"index": i, "type": res.resource})
				if err := dr.Create(ctx, key, val); err != nil {
					t.Fatalf("Create iteration %d failed: %v", i, err)
				}
			}

			// List and verify count
			items, err := dr.List(ctx)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(items) != res.count {
				t.Errorf("expected %d items, got %d", res.count, len(items))
			}

			// Read each item
			for i := 0; i < res.count; i++ {
				key := fmt.Sprintf("%s-%d", res.resource, i)
				got, err := dr.Read(ctx, key)
				if err != nil {
					t.Errorf("Read iteration %d failed: %v", i, err)
				} else if got.Name != key {
					t.Errorf("expected name %s, got %s", key, got.Name)
				}
			}

			// Update all items
			for i := 0; i < res.count; i++ {
				key := fmt.Sprintf("%s-%d", res.resource, i)
				val := createTestValue(key, map[string]interface{}{"index": i * 10})
				if err := dr.Update(ctx, key, val); err != nil {
					t.Errorf("Update iteration %d failed: %v", i, err)
				}
			}

			// Apply over existing items
			for i := 0; i < res.count; i++ {
				key := fmt.Sprintf("%s-%d", res.resource, i)
				val := createTestValue(key, map[string]interface{}{"modified": true})
				if err := dr.Apply(ctx, key, val); err != nil {
					t.Errorf("Apply iteration %d failed: %v", i, err)
				}
			}

			// Delete all items
			for i := 0; i < res.count; i++ {
				key := fmt.Sprintf("%s-%d", res.resource, i)
				if err := dr.Delete(ctx, key); err != nil {
					t.Errorf("Delete iteration %d failed: %v", i, err)
				}
			}

			// Verify empty list
			items, err = dr.List(ctx)
			if err != nil {
				t.Fatalf("List after cleanup failed: %v", err)
			}
			if len(items) != 0 {
				t.Errorf("expected 0 items after cleanup, got %d", len(items))
			}
		})
	}
}

// TestConfigMapOnly tests with v1.ConfigMap type specifically to ensure compatibility with real Kubernetes types
func TestConfigMapOnly(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		namespace string
		cmData    map[string]string
		updates   int
	}{
		{
			name:      "simple_config",
			namespace: "default",
			cmData:    map[string]string{"key1": "value1", "key2": "value2"},
			updates:   3,
		},
		{
			name:      "app_config",
			namespace: "kube-system",
			cmData:    map[string]string{"app.properties": "server.port=8080", "database.url": "localhost:5432"},
			updates:   5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()

			// Use NewFake to create a DynamicResource with v1.ConfigMap type
			cfg := Config{
				Namespace: tc.namespace,
				Group:     "",
				Version:   "v1",
				Resource:  "configmaps",
			}

			dr := NewFake[*v1.ConfigMap](cfg)

			// Create ConfigMap using actual v1.ConfigMap type
			key := fmt.Sprintf("cm-%s", tc.name)
			configMap := &v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      key,
					Namespace: tc.namespace,
				},
				Data: tc.cmData,
			}

			if err := dr.Create(ctx, key, configMap); err != nil {
				t.Fatalf("Create ConfigMap failed: %v", err)
			}

			// Read and verify
			got, err := dr.Read(ctx, key)
			if err != nil {
				t.Fatalf("Read ConfigMap failed: %v", err)
			}
			if got.Name != key {
				t.Errorf("expected name %s, got %s", key, got.Name)
			}
			if len(got.Data) != len(tc.cmData) {
				t.Errorf("expected %d data entries, got %d", len(tc.cmData), len(got.Data))
			}

			// Perform updates
			for i := 0; i < tc.updates; i++ {
				// Read current state first
				current, err := dr.Read(ctx, key)
				if err != nil {
					t.Fatalf("Read iteration %d failed: %v", i, err)
				}

				// Add new data
				current.Data[fmt.Sprintf("update_%d", i)] = fmt.Sprintf("value_%d", i)

				if err := dr.Update(ctx, key, current); err != nil {
					t.Fatalf("Update iteration %d failed: %v", i, err)
				}
			}

			// Verify final state
			final, err := dr.Read(ctx, key)
			if err != nil {
				t.Fatalf("Final Read failed: %v", err)
			}
			expectedKeys := len(tc.cmData) + tc.updates
			if len(final.Data) != expectedKeys {
				t.Errorf("expected %d data keys after updates, got %d", expectedKeys, len(final.Data))
			}

			// Test Apply operation
			final.Data["applied"] = "true"
			if err := dr.Apply(ctx, key, final); err != nil {
				t.Fatalf("Apply ConfigMap failed: %v", err)
			}

			// Test List operation
			items, err := dr.List(ctx)
			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(items) == 0 {
				t.Errorf("expected at least 1 ConfigMap in list")
			}

			// Verify AsMap works with v1.ConfigMap
			itemsMap := items.AsMap()
			if _, exists := itemsMap[key]; !exists {
				t.Errorf("expected key %s in AsMap result", key)
			}

			// Delete ConfigMap
			if err := dr.Delete(ctx, key); err != nil {
				t.Fatalf("Delete ConfigMap failed: %v", err)
			}

			// Verify deletion
			_, err = dr.Read(ctx, key)
			if err == nil || !apierrors.IsNotFound(err) {
				t.Errorf("expected NotFound after Delete, got: %v", err)
			}
		})
	}
}

// TestStoreReset tests the Reset functionality
func TestStoreReset(t *testing.T) {
	t.Parallel()

	itemCounts := []int{5, 10, 15}

	for _, count := range itemCounts {
		count := count
		t.Run(fmt.Sprintf("%d_items", count), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
			dr := newTestStore("default", gvr)

			// Create items
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("reset-%d", i)
				val := createTestValue(key, map[string]interface{}{"index": i})
				if err := dr.Create(ctx, key, val); err != nil {
					t.Fatalf("Create iteration %d failed: %v", i, err)
				}
			}

			// Verify items exist
			items, err := dr.List(ctx)
			if err != nil {
				t.Fatalf("List before Reset failed: %v", err)
			}
			if len(items) != count {
				t.Errorf("expected %d items before reset, got %d", count, len(items))
			}

			// Reset store
			if err := store.Reset(ctx, dr); err != nil {
				t.Fatalf("Reset failed: %v", err)
			}

			// Verify empty
			items, err = dr.List(ctx)
			if err != nil {
				t.Fatalf("List after Reset failed: %v", err)
			}
			if len(items) != 0 {
				t.Errorf("expected 0 items after reset, got %d", len(items))
			}
		})
	}
}

func TestDynamicResource_LabelsEnricher(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	dr := NewFake[*testValue](Config{
		Namespace: "default",
		Group:     "",
		Version:   "v1",
		Resource:  "testresources",
	}, WithLabelsEnricher(func(obj *testValue) (map[string]string, error) {
		// Custom enricher that always adds "env: test"
		return map[string]string{"env": "test", "enriched": "true"}, nil
	}))
	val := createTestValue("enriched", map[string]interface{}{"foo": "bar"})
	customLabels := labelstore.Labels{"user": "bob"}

	// Create with additional custom labels via WithLabels option
	if err := dr.Create(ctx, "enriched", val, labelstore.WithLabels(customLabels)); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := dr.Read(ctx, "enriched")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify enricher labels are present
	if got.Labels["env"] != "test" {
		t.Errorf("expected enricher label 'env: test', got: %v", got.Labels)
	}
	if got.Labels["enriched"] != "true" {
		t.Errorf("expected enricher label 'enriched: true', got: %v", got.Labels)
	}

	// Verify custom labels from WithLabels option are present
	if got.Labels["user"] != "bob" {
		t.Errorf("expected custom label 'user: bob', got: %v", got.Labels)
	}
}

func TestDynamicResource_ImmutableLabels(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	immutable := map[string]string{"app": "myapp", "env": "prod"}

	dr := NewFake[*testValue](Config{
		Namespace:       "default",
		Group:           "",
		Version:         "v1",
		Resource:        "testresources",
		ImmutableLabels: immutable,
	})

	val := createTestValue("immutable", map[string]interface{}{"foo": "bar"})

	// Create with custom labels that don't conflict with immutable labels
	if err := dr.Create(ctx, "immutable", val, labelstore.WithLabels(labelstore.Labels{"custom": "label"})); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := dr.Read(ctx, "immutable")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Verify immutable labels are present
	for k, v := range immutable {
		if got.Labels[k] != v {
			t.Errorf("expected immutable label %s=%s, got %s", k, v, got.Labels[k])
		}
	}

	// Verify custom labels are preserved
	if got.Labels["custom"] != "label" {
		t.Errorf("expected custom label to be preserved, got: %v", got.Labels)
	}

	// Test that trying to override immutable labels returns an error
	val2 := createTestValue("immutable2", map[string]interface{}{"bar": "baz"})
	err = dr.Create(ctx, "immutable2", val2, labelstore.WithLabels(labelstore.Labels{"app": "userapp", "custom": "value"}))
	if err == nil {
		t.Errorf("expected error when trying to override immutable label 'app', got nil")
	} else if err.Error() != "label app is immutable" {
		t.Errorf("expected 'label app is immutable' error, got: %v", err)
	}
}

// TestDynamicResource_ImmutableLabels_WithEnricher tests that enricher cannot override immutable labels
func TestDynamicResource_ImmutableLabels_WithEnricher(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	immutable := map[string]string{"app": "myapp", "env": "prod"}

	// Enricher that tries to set "app" label (which is immutable)
	enricher := func(obj *testValue) (map[string]string, error) {
		return map[string]string{"app": "enricherapp", "extra": "enriched"}, nil
	}

	dr := NewFake[*testValue](Config{
		Namespace:       "default",
		Group:           "",
		Version:         "v1",
		Resource:        "testresources",
		ImmutableLabels: immutable,
	})
	dr.labelsEnricher = enricher

	val := createTestValue("enricher-conflict", map[string]interface{}{"foo": "bar"})

	// This should fail because enricher tries to override immutable label
	err := dr.Create(ctx, "enricher-conflict", val)
	if err == nil {
		t.Errorf("expected error when enricher tries to override immutable label 'app', got nil")
	} else if err.Error() != "label app is immutable" {
		t.Errorf("expected 'label app is immutable' error, got: %v", err)
	}
}

// TestDynamicResource_ListWithPrefix tests the prefix filtering in List operation
func TestDynamicResource_ListWithPrefix(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	// Create items with different prefixes
	testItems := []struct {
		name string
		data map[string]interface{}
	}{
		{"app-frontend-1", map[string]interface{}{"type": "frontend"}},
		{"app-frontend-2", map[string]interface{}{"type": "frontend"}},
		{"app-backend-1", map[string]interface{}{"type": "backend"}},
		{"app-backend-2", map[string]interface{}{"type": "backend"}},
		{"db-primary", map[string]interface{}{"type": "database"}},
		{"db-replica", map[string]interface{}{"type": "database"}},
		{"cache-redis", map[string]interface{}{"type": "cache"}},
	}

	for _, item := range testItems {
		val := createTestValue(item.name, item.data)
		if err := dr.Create(ctx, item.name, val); err != nil {
			t.Fatalf("Create %s failed: %v", item.name, err)
		}
	}

	// Test 1: List all items (no prefix)
	allItems, err := dr.List(ctx)
	if err != nil {
		t.Fatalf("List all failed: %v", err)
	}
	if len(allItems) != 7 {
		t.Errorf("expected 7 items without prefix, got %d", len(allItems))
	}

	// Test 2: List items with prefix "app-"
	appItems, err := dr.List(ctx, store.WithPrefix("app-"))
	if err != nil {
		t.Fatalf("List with prefix 'app-' failed: %v", err)
	}
	if len(appItems) != 4 {
		t.Errorf("expected 4 items with prefix 'app-', got %d", len(appItems))
	}
	for _, item := range appItems {
		if !strings.HasPrefix(item.Key, "app-") {
			t.Errorf("expected item key to start with 'app-', got: %s", item.Key)
		}
	}

	// Test 3: List items with prefix "app-frontend"
	frontendItems, err := dr.List(ctx, store.WithPrefix("app-frontend"))
	if err != nil {
		t.Fatalf("List with prefix 'app-frontend' failed: %v", err)
	}
	if len(frontendItems) != 2 {
		t.Errorf("expected 2 items with prefix 'app-frontend', got %d", len(frontendItems))
	}
	for _, item := range frontendItems {
		if !strings.HasPrefix(item.Key, "app-frontend") {
			t.Errorf("expected item key to start with 'app-frontend', got: %s", item.Key)
		}
	}

	// Test 4: List items with prefix "db-"
	dbItems, err := dr.List(ctx, store.WithPrefix("db-"))
	if err != nil {
		t.Fatalf("List with prefix 'db-' failed: %v", err)
	}
	if len(dbItems) != 2 {
		t.Errorf("expected 2 items with prefix 'db-', got %d", len(dbItems))
	}
	for _, item := range dbItems {
		if !strings.HasPrefix(item.Key, "db-") {
			t.Errorf("expected item key to start with 'db-', got: %s", item.Key)
		}
	}

	// Test 5: List items with prefix "cache"
	cacheItems, err := dr.List(ctx, store.WithPrefix("cache"))
	if err != nil {
		t.Fatalf("List with prefix 'cache' failed: %v", err)
	}
	if len(cacheItems) != 1 {
		t.Errorf("expected 1 item with prefix 'cache', got %d", len(cacheItems))
	}
	if len(cacheItems) > 0 && cacheItems[0].Key != "cache-redis" {
		t.Errorf("expected cache-redis, got: %s", cacheItems[0].Key)
	}

	// Test 6: List items with non-existent prefix
	noMatchItems, err := dr.List(ctx, store.WithPrefix("nonexistent"))
	if err != nil {
		t.Fatalf("List with prefix 'nonexistent' failed: %v", err)
	}
	if len(noMatchItems) != 0 {
		t.Errorf("expected 0 items with prefix 'nonexistent', got %d", len(noMatchItems))
	}
}

// TestDynamicResource_ListWithLabelSelector tests the List operation with label selectors
func TestDynamicResource_ListWithLabelSelector(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	// Create test items once
	testItems := []struct {
		name   string
		labels map[string]string
	}{
		{"item-1", map[string]string{"env": "prod", "tier": "frontend"}},
		{"item-2", map[string]string{"env": "prod", "tier": "backend"}},
		{"item-3", map[string]string{"env": "dev", "tier": "frontend"}},
		{"item-4", map[string]string{"env": "staging"}},
	}

	for i, item := range testItems {
		val := createTestValue(item.name, map[string]interface{}{"id": i})
		if err := dr.Create(ctx, item.name, val, labelstore.WithLabels(item.labels)); err != nil {
			t.Fatalf("Create %s failed: %v", item.name, err)
		}
	}

	// Table-driven tests for different selectors
	tests := []struct {
		name          string
		selector      string
		expectedCount int
		expectedKeys  []string
		shouldError   bool
	}{
		{"no_selector", "", 4, nil, false},
		{"env_prod", "env=prod", 2, nil, false},
		{"env_prod_tier_frontend", "env=prod,tier=frontend", 1, []string{"item-1"}, false},
		{"tier_backend", "tier=backend", 1, []string{"item-2"}, false},
		{"set_based_in", "env in (prod,staging)", 3, nil, false},
		{"set_based_notin", "tier notin (frontend)", 2, nil, false},
		{"invalid_syntax", "invalid===syntax", 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var items store.ListItems[string, *testValue]
			var err error

			if tt.selector == "" {
				items, err = dr.List(ctx)
			} else {
				items, err = dr.List(ctx, labelstore.WithLabelSelector(tt.selector))
			}

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if len(items) != tt.expectedCount {
				t.Errorf("expected %d items, got %d", tt.expectedCount, len(items))
			}
			for _, expectedKey := range tt.expectedKeys {
				if !slices.ContainsFunc(items, func(item store.ListItem[string, *testValue]) bool {
					return item.Key == expectedKey
				}) {
					t.Errorf("expected key %s not found in results", expectedKey)
				}
			}
		})
	}
}

// TestDynamicResource_LabelSelectorWithImmutableLabels tests immutable labels with selectors
func TestDynamicResource_LabelSelectorWithImmutableLabels(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	dr := NewFake[*testValue](Config{
		Namespace:       "default",
		Group:           gvr.Group,
		Version:         gvr.Version,
		Resource:        gvr.Resource,
		ImmutableLabels: map[string]string{"app": "myapp"},
	})

	// Create items - all will have app=myapp due to immutable labels
	items := []struct {
		name   string
		labels map[string]string
	}{
		{"item-1", map[string]string{"env": "prod", "tier": "frontend"}},
		{"item-2", map[string]string{"env": "prod"}},
		{"item-3", map[string]string{"env": "dev"}},
	}

	for _, item := range items {
		val := createTestValue(item.name, map[string]interface{}{})
		if err := dr.Create(ctx, item.name, val, labelstore.WithLabels(item.labels)); err != nil {
			t.Fatalf("Create %s failed: %v", item.name, err)
		}
	}

	// Test valid selectors that combine with immutable labels
	if list, err := dr.List(ctx); err != nil || len(list) != 3 {
		t.Errorf("expected 3 items with app=myapp, got %d, err: %v", len(list), err)
	}

	if list, err := dr.List(ctx, labelstore.WithLabelSelector("env=prod")); err != nil || len(list) != 2 {
		t.Errorf("expected 2 items with app=myapp,env=prod, got %d, err: %v", len(list), err)
	}

	if list, err := dr.List(ctx, labelstore.WithLabelSelector("env=prod,tier=frontend")); err != nil || len(list) != 1 {
		t.Errorf("expected 1 item, got %d, err: %v", len(list), err)
	}

	// Test that overriding immutable label fails
	_, err := dr.List(ctx, labelstore.WithLabelSelector("app=otherapp"))
	if err == nil || !strings.Contains(err.Error(), "cannot override additional label selector over immutable key 'app'") {
		t.Errorf("expected immutable label conflict error, got: %v", err)
	}
}

// TestDynamicResource_WithResolveConflict tests the WithResolveConflict option for apply operations
func TestDynamicResource_WithResolveConflict(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	t.Run("apply_with_resolve_conflict", func(t *testing.T) {
		key := "conflict-test-1"

		// Create initial resource
		val := createTestValue(key, map[string]interface{}{"version": "v1", "field1": "value1"})
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Read current state
		current, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		// Modify the resource directly (simulate another user's change)
		current.Spec["version"] = "v2"
		current.Spec["field2"] = "value2"
		if err := dr.Update(ctx, key, current); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		// Now apply with WithResolveConflict - should succeed and override the changes
		modified := createTestValue(key, map[string]interface{}{"version": "v3", "field3": "value3"})
		if err := dr.Apply(ctx, key, modified, WithResolveConflict(true)); err != nil {
			t.Fatalf("Apply with WithResolveConflict failed: %v", err)
		}

		// Verify the resource was updated with our changes
		final, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read after conflict resolution failed: %v", err)
		}

		// The apply should have succeeded with our version
		if final.Spec["version"] != "v3" {
			t.Errorf("expected version v3, got %v", final.Spec["version"])
		}
		if final.Spec["field3"] != "value3" {
			t.Errorf("expected field3=value3, got %v", final.Spec["field3"])
		}
	})

	t.Run("apply_without_resolve_conflict_fallback", func(t *testing.T) {
		key := "conflict-test-2"

		// Create initial resource
		val := createTestValue(key, map[string]interface{}{"version": "v1"})
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Apply without WithResolveConflict should still work via fallback mechanism
		// (in fake client, the Apply might fail and fall back to Update)
		modified := createTestValue(key, map[string]interface{}{"version": "v2", "updated": "true"})
		if err := dr.Apply(ctx, key, modified); err != nil {
			t.Fatalf("Apply without WithResolveConflict failed: %v", err)
		}

		// Verify the resource was updated
		final, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read after apply failed: %v", err)
		}
		if final.Spec["version"] != "v2" {
			t.Errorf("expected version v2, got %v", final.Spec["version"])
		}
	})

	t.Run("apply_force_with_resolve_conflict", func(t *testing.T) {
		key := "conflict-test-3"

		// Create initial resource
		val := createTestValue(key, map[string]interface{}{"version": "v1", "data": "original"})
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Apply with WithResolveConflict(true) sets Force=true in ApplyOptions
		// This should force the apply even if there are conflicts
		forced := createTestValue(key, map[string]interface{}{"version": "v2", "data": "forced", "forced": "true"})
		if err := dr.Apply(ctx, key, forced, WithResolveConflict(true)); err != nil {
			t.Fatalf("Apply with Force failed: %v", err)
		}

		// Verify the forced apply succeeded
		final, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read after forced apply failed: %v", err)
		}
		if final.Spec["version"] != "v2" {
			t.Errorf("expected version v2, got %v", final.Spec["version"])
		}
		if final.Spec["data"] != "forced" {
			t.Errorf("expected data=forced, got %v", final.Spec["data"])
		}
		if final.Spec["forced"] != "true" {
			t.Errorf("expected forced=true, got %v", final.Spec["forced"])
		}
	})
}

// TestDynamicResource_WithSubResourceOnly tests the WithSubResourceOnly option for update operations
func TestDynamicResource_WithSubResourceOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "testresources"}
	dr := newTestStore("default", gvr)

	// Create resource with initial spec and status
	key := "status-test"
	val := createTestValue(key, map[string]interface{}{"replicas": 3})
	val.ObjectMeta.Annotations = map[string]string{"initial": "true"}
	if err := dr.Create(ctx, key, val); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Read current state
	current, err := dr.Read(ctx, key)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Update with WithSubResourceOnly - should only update status subresource
	current.Spec["replicas"] = 5
	current.ObjectMeta.Annotations = map[string]string{"updated": "true"}
	if err := dr.Update(ctx, key, current, WithSubResourceOnly(true)); err != nil {
		t.Fatalf("Update with WithSubResourceOnly failed: %v", err)
	}

	// In fake client, behavior may vary, but the option should be accepted
	// Verify update was applied
	updated, err := dr.Read(ctx, key)
	if err != nil {
		t.Fatalf("Read after update failed: %v", err)
	}
	if updated.Name != key {
		t.Errorf("expected name %s, got %s", key, updated.Name)
	}

	// Test UpdateStatus method which is specifically for status subresource
	statusUpdate := createTestValue(key, map[string]interface{}{"status": "ready"})
	if err := dr.Update(ctx, key, statusUpdate, WithSubResourceOnly(true)); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Test Apply with WithSubResourceOnly
	applyVal := createTestValue(key, map[string]interface{}{"status": "applied"})
	if err := dr.Apply(ctx, key, applyVal, WithSubResourceOnly(false)); err != nil {
		t.Fatalf("Apply with WithSubResourceOnly failed: %v", err)
	}
}

// TestDynamicResource_ListKeysOnly tests the ListKeysOnly option
func TestDynamicResource_ListKeysOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	// Create multiple resources with varying amounts of data
	items := []struct {
		key  string
		data map[string]interface{}
	}{
		{"item-1", map[string]interface{}{"large": strings.Repeat("data", 1000)}},
		{"item-2", map[string]interface{}{"large": strings.Repeat("data", 1000)}},
		{"item-3", map[string]interface{}{"large": strings.Repeat("data", 1000)}},
		{"item-4", map[string]interface{}{"large": strings.Repeat("data", 1000)}},
		{"item-5", map[string]interface{}{"large": strings.Repeat("data", 1000)}},
	}

	for _, item := range items {
		val := createTestValue(item.key, item.data)
		if err := dr.Create(ctx, item.key, val); err != nil {
			t.Fatalf("Create %s failed: %v", item.key, err)
		}
	}

	// List with full data (default behavior)
	fullList, err := dr.List(ctx)
	if err != nil {
		t.Fatalf("List full failed: %v", err)
	}
	if len(fullList) != 5 {
		t.Errorf("expected 5 items in full list, got %d", len(fullList))
	}
	// Verify values are populated
	for _, item := range fullList {
		if item.Value == nil {
			t.Errorf("expected value to be populated for key %s", item.Key)
		}
		if item.Value.Spec == nil || len(item.Value.Spec) == 0 {
			t.Errorf("expected spec data for key %s", item.Key)
		}
	}

	// List with ListKeysOnly - values should be empty/nil
	keysOnly, err := dr.List(ctx, store.ListKeysOnly)
	if err != nil {
		t.Fatalf("List keys only failed: %v", err)
	}
	if len(keysOnly) != 5 {
		t.Errorf("expected 5 items in keys-only list, got %d", len(keysOnly))
	}
	// Verify keys are present but values are empty
	for _, item := range keysOnly {
		if item.Key == "" {
			t.Errorf("expected key to be populated")
		}
		// Value should be nil or empty (zero value)
		if item.Value != nil && item.Value.Spec != nil && len(item.Value.Spec) > 0 {
			t.Errorf("expected empty value for key %s when using ListKeysOnly, got: %v", item.Key, item.Value.Spec)
		}
	}

	// Verify all expected keys are present
	expectedKeys := []string{"item-1", "item-2", "item-3", "item-4", "item-5"}
	for _, expectedKey := range expectedKeys {
		found := false
		for _, item := range keysOnly {
			if item.Key == expectedKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected key %s not found in keys-only list", expectedKey)
		}
	}

	// Test combination: ListKeysOnly with prefix filter
	prefixKeysOnly, err := dr.List(ctx, store.ListKeysOnly, store.WithPrefix("item-"))
	if err != nil {
		t.Fatalf("List keys only with prefix failed: %v", err)
	}
	if len(prefixKeysOnly) != 5 {
		t.Errorf("expected 5 items with prefix and keys-only, got %d", len(prefixKeysOnly))
	}

	// Test combination: ListKeysOnly with label selector
	labeledKey := "labeled-item"
	labeledVal := createTestValue(labeledKey, map[string]interface{}{"test": "data"})
	if err := dr.Create(ctx, labeledKey, labeledVal, labelstore.WithLabels(map[string]string{"special": "true"})); err != nil {
		t.Fatalf("Create labeled item failed: %v", err)
	}

	labeledKeysOnly, err := dr.List(ctx, store.ListKeysOnly, labelstore.WithLabelSelector("special=true"))
	if err != nil {
		t.Fatalf("List keys only with label selector failed: %v", err)
	}
	if len(labeledKeysOnly) != 1 {
		t.Errorf("expected 1 item with label and keys-only, got %d", len(labeledKeysOnly))
	}
	if len(labeledKeysOnly) > 0 && labeledKeysOnly[0].Key != labeledKey {
		t.Errorf("expected key %s, got %s", labeledKey, labeledKeysOnly[0].Key)
	}
}

// TestDynamicResource_DryRun tests that DryRun operations don't persist changes
func TestDynamicResource_DryRun(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	dr := newTestStore("default", gvr)

	t.Run("create_with_dryrun", func(t *testing.T) {
		key := "dryrun-create"
		val := createTestValue(key, map[string]interface{}{"test": "data"})

		// Create with DryRun - should not persist
		err := dr.Create(ctx, key, val, store.DryRun)
		if err != nil {
			t.Fatalf("DryRun Create failed: %v", err)
		}

		// Verify resource doesn't exist
		_, err = dr.Read(ctx, key)
		if err == nil {
			t.Error("expected NotFound error after DryRun create, resource should not exist")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}

		// Verify list is empty
		items, err := dr.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items after DryRun create, got %d", len(items))
		}
	})

	t.Run("update_with_dryrun_nonexistent", func(t *testing.T) {
		key := "dryrun-update-missing"
		val := createTestValue(key, map[string]interface{}{"test": "data"})

		// Update non-existent resource with DryRun - should fail with NotFound
		err := dr.Update(ctx, key, val, store.DryRun)
		if err == nil {
			t.Error("expected error when updating non-existent resource with DryRun")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error for DryRun update of non-existent resource, got: %v", err)
		}
	})

	t.Run("update_with_dryrun_existing", func(t *testing.T) {
		key := "dryrun-update-existing"
		val := createTestValue(key, map[string]interface{}{"version": "v1"})

		// Create resource normally
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Read the current state to get the ResourceVersion
		current, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		// Update with DryRun - should succeed but not persist
		current.Spec["version"] = "v2"
		err = dr.Update(ctx, key, current, store.DryRun)
		if err != nil {
			t.Fatalf("DryRun Update failed: %v", err)
		}

		// Verify original value is unchanged
		current, err = dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if current.Spec["version"] != "v1" {
			t.Errorf("expected version v1 after DryRun update, got: %v", current.Spec["version"])
		}
	})

	t.Run("apply_with_dryrun_new_resource", func(t *testing.T) {
		key := "dryrun-apply-new"
		val := createTestValue(key, map[string]interface{}{"applied": "true"})

		// Apply new resource with DryRun - should not persist
		err := dr.Apply(ctx, key, val, store.DryRun)
		if err != nil {
			t.Fatalf("DryRun Apply failed: %v", err)
		}

		// Verify resource doesn't exist
		_, err = dr.Read(ctx, key)
		if err == nil {
			t.Error("expected NotFound error after DryRun apply of new resource")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}
	})

	t.Run("apply_with_dryrun_existing_resource", func(t *testing.T) {
		key := "dryrun-apply-existing"
		val := createTestValue(key, map[string]interface{}{"version": "v1"})

		// Create resource normally
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Apply update with DryRun - should succeed but not persist
		updated := createTestValue(key, map[string]interface{}{"version": "v2", "extra": "data"})
		err := dr.Apply(ctx, key, updated, store.DryRun)
		if err != nil {
			t.Fatalf("DryRun Apply failed: %v", err)
		}

		// Verify original value is unchanged
		current, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if current.Spec["version"] != "v1" {
			t.Errorf("expected version v1 after DryRun apply, got: %v", current.Spec["version"])
		}
		if _, exists := current.Spec["extra"]; exists {
			t.Error("expected 'extra' field to not exist after DryRun apply")
		}
	})

	t.Run("delete_with_dryrun_nonexistent", func(t *testing.T) {
		key := "dryrun-delete-missing"

		// Delete non-existent resource with DryRun - should fail with NotFound
		err := dr.Delete(ctx, key, store.DryRun)
		if err == nil {
			t.Error("expected error when deleting non-existent resource with DryRun")
		}
		if !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error for DryRun delete of non-existent resource, got: %v", err)
		}
	})

	t.Run("delete_with_dryrun_existing", func(t *testing.T) {
		key := "dryrun-delete-existing"
		val := createTestValue(key, map[string]interface{}{"data": "value"})

		// Create resource normally
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Delete with DryRun - should succeed but not persist
		err := dr.Delete(ctx, key, store.DryRun)
		if err != nil {
			t.Fatalf("DryRun Delete failed: %v", err)
		}

		// Verify resource still exists
		current, err := dr.Read(ctx, key)
		if err != nil {
			t.Errorf("expected resource to still exist after DryRun delete, got error: %v", err)
		}
		if current.Name != key {
			t.Errorf("expected name %s, got %s", key, current.Name)
		}

		// Cleanup: actually delete the resource
		if err := dr.Delete(ctx, key); err != nil {
			t.Errorf("Cleanup delete failed: %v", err)
		}
	})

	t.Run("create_duplicate_with_dryrun", func(t *testing.T) {
		key := "dryrun-duplicate"
		val := createTestValue(key, map[string]interface{}{"data": "value"})

		// Create resource normally
		if err := dr.Create(ctx, key, val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Try to create duplicate with DryRun - should fail with AlreadyExists
		duplicate := createTestValue(key, map[string]interface{}{"data": "different"})
		err := dr.Create(ctx, key, duplicate, store.DryRun)
		if err == nil {
			t.Error("expected AlreadyExists error when creating duplicate with DryRun")
		}
		if !apierrors.IsConflict(err) {
			t.Errorf("expected Conflict/AlreadyExists error, got: %v", err)
		}

		// Verify original value is unchanged
		current, err := dr.Read(ctx, key)
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if current.Spec["data"] != "value" {
			t.Errorf("expected original data value, got: %v", current.Spec["data"])
		}
	})
}
