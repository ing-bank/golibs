package cache

import (
	"context"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func TestSetAndGet(t *testing.T) {
	t.Parallel()
	persist := memory.NewOrDie[string, string]()
	cache := memory.NewOrDie[string, string]()
	c, err := New(persist, cache)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	err = c.Apply(t.Context(), "a", "1")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Force cache read
	c.(*Store[string, string]).store = nil

	// Check via cache.Store API
	val, err := c.Read(t.Context(), "a")
	if err != nil || val != "1" {
		t.Fatalf("cache.Store Read failed: got %v, err %v", val, err)
	}
	// Check persistent store
	valP, errP := persist.Read(t.Context(), "a")
	if errP != nil || valP != "1" {
		t.Fatalf("Persistent store get failed: got %v, err %v", valP, errP)
	}
	// Check cache store
	valC, errC := cache.Read(t.Context(), "a")
	if errC != nil || valC != "1" {
		t.Fatalf("Cache store get failed: got %v, err %v", valC, errC)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	persist := memory.NewOrDie[string, string]()
	cache := memory.NewOrDie[string, string]()
	c, err := New(persist, cache)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	_ = c.Apply(t.Context(), "a", "1")
	err = c.Delete(t.Context(), "a")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	// Check via cache.Store API
	_, err = c.Read(t.Context(), "a")
	if err == nil {
		t.Fatalf("Expected not found for deleted key via cache.Store API")
	}
	_, errP := persist.Read(t.Context(), "a")
	if errP == nil {
		t.Fatalf("Expected not found for deleted key in persistent store")
	}
	_, errC := cache.Read(t.Context(), "a")
	if errC == nil {
		t.Fatalf("Expected not found for deleted key in cache store")
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	persist := memory.NewOrDie[string, string]()
	cache := memory.NewOrDie[string, string]()
	c, err := New(persist, cache)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	_ = c.Apply(t.Context(), "a", "1")
	_ = c.Apply(t.Context(), "b", "2")
	_ = store.Reset(t.Context(), c)
	// Check via cache.Store API
	_, errA := c.Read(t.Context(), "a")
	if errA == nil {
		t.Fatalf("Expected not found for key 'a' after reset via cache.Store API")
	}
	_, errB := c.Read(t.Context(), "b")
	if errB == nil {
		t.Fatalf("Expected not found for key 'b' after reset via cache.Store API")
	}
	_, errPA := persist.Read(t.Context(), "a")
	if errPA == nil {
		t.Fatalf("Expected not found for key 'a' after reset in persistent store")
	}
	_, errPB := persist.Read(t.Context(), "b")
	if errPB == nil {
		t.Fatalf("Expected not found for key 'b' after reset in persistent store")
	}
	_, errCA := cache.Read(t.Context(), "a")
	if errCA == nil {
		t.Fatalf("Expected not found for key 'a' after reset in cache store")
	}
	_, errCB := cache.Read(t.Context(), "b")
	if errCB == nil {
		t.Fatalf("Expected not found for key 'b' after reset in cache store")
	}
}

func TestGetNonExistentReturnsNotFound(t *testing.T) {
	t.Parallel()
	persist := memory.NewOrDie[string, string]()
	cache := memory.NewOrDie[string, string]()
	c, err := New(persist, cache)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	// Check via cache.Store API
	_, err = c.Read(t.Context(), "nope")
	if err == nil {
		t.Fatalf("Expected not found for non-existent key via cache.Store API")
	}
	_, errP := persist.Read(t.Context(), "nope")
	if errP == nil {
		t.Fatalf("Expected not found for non-existent key in persistent store")
	}
	_, errC := cache.Read(t.Context(), "nope")
	if errC == nil {
		t.Fatalf("Expected not found for non-existent key in cache store")
	}
}

func TestCache_Iterate(t *testing.T) {
	persist := memory.NewOrDie[string, string]()
	cache := memory.NewOrDie[string, string]()
	c, err := New(persist, cache)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	_ = c.Apply(context.Background(), "a", "1")
	_ = c.Apply(context.Background(), "b", "2")

	// Check via cache.Store API
	items, _ := c.List(context.Background())
	keys := items.AsMap()
	if len(keys) != 2 {
		t.Errorf("Iterate failed, expected 2 items via cache.Store API, got %d", len(keys))
	}
	if keys["a"] != "1" || keys["b"] != "2" {
		t.Errorf("Iterate failed, unexpected values via cache.Store API: %v", keys)
	}

	itemsP, _ := persist.List(context.Background())
	keysP := itemsP.AsMap()
	itemsC, _ := cache.List(context.Background())
	keysC := itemsC.AsMap()

	// Check that we got both items in persistent store
	if len(keysP) != 2 {
		t.Errorf("Iterate failed, expected 2 items in persistent store, got %d", len(keysP))
	}
	if keysP["a"] != "1" || keysP["b"] != "2" {
		t.Errorf("Iterate failed, unexpected values in persistent store: %v", keysP)
	}
	// Check that we got both items in cache store
	if len(keysC) != 2 {
		t.Errorf("Iterate failed, expected 2 items in cache store, got %d", len(keysC))
	}
	if keysC["a"] != "1" || keysC["b"] != "2" {
		t.Errorf("Iterate failed, unexpected values in cache store: %v", keysC)
	}
}
