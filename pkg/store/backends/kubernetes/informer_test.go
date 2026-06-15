package kubernetes

import (
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCachedDynamicResource_Read(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Create a test object to pre-populate the cache
	val := createTestValue("foobar", map[string]interface{}{"foo": "bar"})

	// Convert to unstructured for the fake client
	unstructuredVal, err := runtime.DefaultUnstructuredConverter.ToUnstructured(val)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}
	initialObj := &unstructured.Unstructured{Object: unstructuredVal}
	initialObj.SetNamespace("default")
	initialObj.SetName("foobar")
	initialObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "testresource",
	})

	// Create cached store with fake client and initial object
	cached := NewCachedFake[*testValue](CachedConfig{
		Config: Config{
			Namespace: "default",
			Group:     "",
			Version:   "v1",
			Resource:  "testresources",
		},
		//ResyncPeriod:       types.Duration(100 * time.Millisecond),
		WaitForCacheToSync: true, // set default to true
	}, initialObj)
	defer cached.Stop()

	t.Run("read_from_cache", func(t *testing.T) {
		//t.Parallel()

		val := createTestValue("foobar2", map[string]interface{}{"foo": "bar"})
		if err := cached.Create(ctx, "foobar2", val); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Read from cache (default behavior)
		got, err := cached.Read(ctx, "foobar2")
		if err != nil && !errors.IsNotFound(err) {
			t.Fatalf("unexpected error on first read: %v", err)
		}

		// Ensure cache has time to update
		time.Sleep(1 * time.Second)

		// Re-Read same item from cache
		got1, err := cached.Read(ctx, "foobar2")
		if err != nil {
			t.Fatalf("Read from cache failed: %v", err)
		}
		if got1.Name != "foobar2" {
			t.Errorf("expected name 'foobar2', got %s", got1.Name)
		}
		if got1.Spec["foo"] != "bar" {
			t.Errorf("expected spec.foo='bar', got %v", got1.Spec["foo"])
		}

		// Re-Read same item from cache
		got2, err := cached.Read(ctx, "foobar")
		if err != nil {
			t.Fatalf("Read from cache failed: %v", err)
		}
		if got2.Name != "foobar" {
			t.Errorf("expected name 'foobar2', got %s", got.Name)
		}
		if got2.Spec["foo"] != "bar" {
			t.Errorf("expected spec.foo='bar', got %v", got.Spec["foo"])
		}

	})

	t.Run("read_with_no_cache", func(t *testing.T) {
		t.Parallel()
		// Read with NoCache option - should bypass cache and hit API
		got, err := cached.Read(ctx, "foobar", store.NoCache)
		if err != nil {
			t.Fatalf("Read with NoCache failed: %v", err)
		}

		if got.Name != "foobar" {
			t.Errorf("expected name 'foobar', got %s", got.Name)
		}

		if got.Spec["foo"] != "bar" {
			t.Errorf("expected spec.foo='bar', got %v", got.Spec["foo"])
		}
	})

	t.Run("read_not_found_from_cache", func(t *testing.T) {
		t.Parallel()
		// Read non-existent item from cache
		_, err := cached.Read(ctx, "nonexistent")
		if err == nil || !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}
	})

	t.Run("read_not_found_with_no_cache", func(t *testing.T) {
		t.Parallel()
		// Read non-existent item with NoCache
		_, err := cached.Read(ctx, "nonexistent", store.NoCache)
		if err == nil || !apierrors.IsNotFound(err) {
			t.Errorf("expected NotFound error, got: %v", err)
		}
	})

	t.Run("read_unsupported_option", func(t *testing.T) {
		t.Parallel()
		// Test that unsupported options return error
		type unsupportedOption string
		_, err := cached.Read(ctx, "foobar", unsupportedOption("test"))
		if err != store.ErrUnsupportedOption {
			t.Errorf("expected ErrUnsupportedOption, got: %v", err)
		}
	})
}
