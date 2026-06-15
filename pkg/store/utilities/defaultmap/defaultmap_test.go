package defaultmap

import (
	"context"
	"testing"
)

func TestDefaultMap_Set(t *testing.T) {
	ctx := context.Background()
	m := New[string, int]()
	m.Apply(ctx, "a", 1)
	val, err := m.Store.Read(ctx, "a")
	if err != nil || val != 1 {
		t.Errorf("expected 1, got %d (err: %v)", val, err)
	}
	m.Apply(ctx, "a", 2)
	val, err = m.Store.Read(ctx, "a")
	if err != nil || val != 2 {
		t.Errorf("expected 2, got %d (err: %v)", val, err)
	}
}

func TestDefaultMap_Update(t *testing.T) {
	ctx := context.Background()
	m := New[string, int]()
	collision := func(old, new int) int { return old + new }
	m.Update(ctx, "a", 1, collision)
	val, err := m.Store.Read(ctx, "a")
	if err != nil || val != 1 {
		t.Errorf("expected 1, got %d (err: %v)", val, err)
	}
	m.Update(ctx, "a", 2, collision)
	val, err = m.Store.Read(ctx, "a")
	if err != nil || val != 3 {
		t.Errorf("expected 3, got %d (err: %v)", val, err)
	}
}

func TestDefaultMap_Merge(t *testing.T) {
	ctx := context.Background()
	m1 := New[string, int]()
	m1.Apply(ctx, "a", 1)
	m1.Apply(ctx, "b", 2)
	m2 := New[string, int]()
	m2.Apply(ctx, "a", 3)
	m2.Apply(ctx, "c", 4)
	collision := func(old, new int) int { return old * new }
	m1.Merge(ctx, m2.Store, collision)
	valA, _ := m1.Store.Read(ctx, "a")
	valB, _ := m1.Store.Read(ctx, "b")
	valC, _ := m1.Store.Read(ctx, "c")
	if valA != 3 {
		t.Errorf("expected 3, got %d", valA)
	}
	if valB != 2 {
		t.Errorf("expected 2, got %d", valB)
	}
	if valC != 4 {
		t.Errorf("expected 4, got %d", valC)
	}
}
