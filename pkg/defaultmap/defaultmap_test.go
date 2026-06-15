package defaultmap

import (
	"testing"
)

func TestDefaultMap_Set(t *testing.T) {
	m := DefaultMap[string, int]{}
	m.Set("a", 1)
	if m["a"] != 1 {
		t.Errorf("expected 1, got %d", m["a"])
	}
	m.Set("a", 2)
	if m["a"] != 2 {
		t.Errorf("expected 2, got %d", m["a"])
	}
}

func TestDefaultMap_Update(t *testing.T) {
	m := DefaultMap[string, int]{}
	collision := func(old, new int) int { return old + new }
	m.Update("a", 1, collision)
	if m["a"] != 1 {
		t.Errorf("expected 1, got %d", m["a"])
	}
	m.Update("a", 2, collision)
	if m["a"] != 3 {
		t.Errorf("expected 3, got %d", m["a"])
	}
}

func TestDefaultMap_Merge(t *testing.T) {
	m1 := DefaultMap[string, int]{"a": 1, "b": 2}
	m2 := DefaultMap[string, int]{"a": 3, "c": 4}
	collision := func(old, new int) int { return old * new }
	m1.Merge(m2, collision)
	if m1["a"] != 3 {
		t.Errorf("expected 3, got %d", m1["a"])
	}
	if m1["b"] != 2 {
		// Testing this case also makes sures the collision function is not applied to non-colliding keys
		t.Errorf("expected 2, got %d", m1["b"])
	}
	if m1["c"] != 4 {
		// Testing this case also makes sures the collision function is not applied to non-colliding keys
		t.Errorf("expected 4, got %d", m1["c"])
	}
}
