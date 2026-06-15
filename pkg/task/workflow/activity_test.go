package workflow

import (
	"context"
	"testing"
)

func TestChain(t *testing.T) {
	noRun := func(ctx context.Context, state int) error { return nil }
	w := NewActivity("w", noRun)
	x := NewActivity("x", noRun)

	// --- Test Describe ---
	c := &Chain[int]{NewActivity("a", noRun), NewActivity("b", noRun)}
	if desc := c.Describe(); desc != "a, b" {
		t.Fatalf("expected chain with 'a, b', but got '%s'", desc)
	}

	// --- Test Weave ---
	if desc := c.Weave(w).Describe(); desc != "a, w, b, w" {
		t.Fatalf("expected chain with 'a, w, b, w', but got '%s'", desc)
	}

	// --- Test list of Weaves ---
	d := NewChain[int](NewActivity("a", noRun), NewActivity("b", noRun)).Weave(w, x)
	if desc := d.Describe(); desc != "a, w, x, b, w, x" {
		t.Fatalf("expected chain with 'a, w, x, b, w, x', but got '%s'", desc)
	}
}
