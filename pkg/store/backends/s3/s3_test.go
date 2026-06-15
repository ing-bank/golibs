package s3

import (
	"context"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
)

func newTestStore() (store.Store[string, string], error) {
	client := NewMockS3Client()
	cfg := &Config[string]{
		Bucket: "test-Bucket",
	}
	return New[string](context.TODO(), client, cfg)
}

func TestCreateAndRead(t *testing.T) {
	db, err := newTestStore()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	ctx := context.Background()
	err = db.Create(ctx, "a", "hello")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	val, err := db.Read(ctx, "a")
	if err != nil || val != "hello" {
		t.Fatalf("Read failed: got %v, err %v", val, err)
	}
}

func TestUpdate(t *testing.T) {
	db, err := newTestStore()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	ctx := context.Background()
	_ = db.Apply(ctx, "a", "foo")
	err = db.Update(ctx, "a", "bar")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	val, _ := db.Read(ctx, "a")
	if val != "bar" {
		t.Fatalf("Update did not persist value, got %v", val)
	}
}

func TestDelete(t *testing.T) {
	db, err := newTestStore()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	ctx := context.Background()
	_ = db.Apply(ctx, "a", "foo")
	err = db.Delete(ctx, "a")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = db.Read(ctx, "a")
	if err == nil {
		t.Fatalf("Expected error for deleted key")
	}
}

func TestList(t *testing.T) {
	db, err := newTestStore()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	ctx := context.Background()
	_ = db.Apply(ctx, "a", "foo")
	_ = db.Apply(ctx, "b", "bar")
	items, err := db.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	m := items.AsMap()
	if m["a"] != "foo" || m["b"] != "bar" {
		t.Fatalf("List returned wrong values: %v", m)
	}
}

func TestListWithPrefix(t *testing.T) {
	db, err := newTestStore()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}
	ctx := context.Background()
	_ = db.Apply(ctx, "pre-1", "a")
	_ = db.Apply(ctx, "pre-2", "b")
	_ = db.Apply(ctx, "other-1", "c")
	_ = db.Apply(ctx, "noprefix", "d")

	// List with prefix 'pre-'
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

	// List with non-existent prefix
	items, err = db.List(ctx, store.WithPrefix("nonexistent-"))
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items with prefix 'nonexistent-', got %d", len(items))
	}
}
