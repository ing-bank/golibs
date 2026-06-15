package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
)

func TestSetAndGet(t *testing.T) {
	t.Parallel()
	c, _ := New[string, string]()
	err := c.Apply(t.Context(), "a", "1")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	val, err := c.Read(t.Context(), "a")
	if err != nil || val != "1" {
		t.Fatalf("Get failed: got %v, err %v", val, err)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	c, _ := New[string, string]()
	_ = c.Apply(t.Context(), "a", "1")
	err := c.Delete(t.Context(), "a")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = c.Read(t.Context(), "a")
	if err == nil {
		t.Fatalf("Expected not found for deleted key")
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	c, _ := New[string, string]()
	_ = c.Apply(t.Context(), "a", "1")
	_ = c.Apply(t.Context(), "b", "2")
	_ = store.Reset(t.Context(), c)
	_, errA := c.Read(t.Context(), "a")
	if errA == nil {
		t.Fatalf("Expected not found for key 'a' after reset")
	}
	_, errB := c.Read(t.Context(), "b")
	if errB == nil {
		t.Fatalf("Expected not found for key 'b' after reset")
	}
}

func TestGetNonExistentReturnsNotFound(t *testing.T) {
	t.Parallel()
	c, _ := New[string, string]()
	_, err := c.Read(t.Context(), "nope")
	if err == nil {
		t.Fatalf("Expected not found for non-existent key")
	}
}

func TestCache_Iterate(t *testing.T) {
	c, _ := New[string, string]()
	_ = c.Apply(context.Background(), "a", "1")
	_ = c.Apply(context.Background(), "b", "2")

	items, _ := c.List(context.Background())
	keys := items.AsMap()

	// Check that we got both items
	if len(keys) != 2 {
		t.Errorf("Iterate failed, expected 2 items, got %d", len(keys))
	}
	// Check that both keys are present
	if keys["a"] != "1" || keys["b"] != "2" {
		t.Errorf("Iterate failed, unexpected values: %v", keys)
	}
}

func TestListWithPrefix_StringKey(t *testing.T) {
	ctx := context.Background()
	db, err := New[string, string]()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	_ = db.Apply(ctx, "pre-1", "a")
	_ = db.Apply(ctx, "pre-2", "b")
	_ = db.Apply(ctx, "other-1", "c")
	_ = db.Apply(ctx, "noprefix", "d")

	items, err := db.List(ctx, store.WithPrefix("pre-"))
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items with prefix 'pre-', got %d", len(items))
	}
	for _, item := range items {
		if len(item.Key) < 4 || item.Key[:4] != "pre-" {
			t.Errorf("expected key with prefix 'pre-', got %s", item.Key)
		}
	}

	items, err = db.List(ctx, store.WithPrefix("nonexistent-"))
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items with prefix 'nonexistent-', got %d", len(items))
	}
}

func TestListWithPrefix_NonStringKey(t *testing.T) {
	ctx := context.Background()
	db, err := New[int, string]()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	_ = db.Apply(ctx, 1, "a")
	_ = db.Apply(ctx, 2, "b")
	_, err = db.List(ctx, store.WithPrefix("pre-"))
	if !errors.Is(err, store.ErrUnsupportedOption) {
		t.Errorf("expected ErrUnsupportedOption for non-string key, got %v", err)
	}
}
