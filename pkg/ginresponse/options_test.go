package ginresponse

import (
	"errors"
	"testing"
)

func TestWithErrorToBody(t *testing.T) {
	customBody := func(err error) any {
		return map[string]string{"custom": err.Error()}
	}
	w, err := NewWrapper(WithErrorToBody(customBody))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testErr := errors.New("test error")
	body := w.ErrorToBody(testErr)
	m, ok := body.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", body)
	}
	if m["custom"] != "test error" {
		t.Errorf("expected custom body, got %v", m)
	}
}

func TestWithErrorToStatus(t *testing.T) {
	customStatus := func(err error) int {
		return 418 // I'm a teapot
	}
	w, err := NewWrapper(WithErrorToStatus(customStatus))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testErr := errors.New("test error")
	status := w.ErrorToStatus(testErr)
	if status != 418 {
		t.Errorf("expected status 418, got %d", status)
	}
}
