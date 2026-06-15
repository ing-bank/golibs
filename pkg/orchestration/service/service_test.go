package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type mockService struct {
	name     string
	validate error
	apply    error
	delete   error
	calls    []string
}

func (m *mockService) Name() string { return m.name }
func (m *mockService) Validate(_ context.Context, _ *Event) error {
	m.calls = append(m.calls, "validate")
	return m.validate
}
func (m *mockService) Apply(_ context.Context, _ *Event) error {
	m.calls = append(m.calls, "apply")
	return m.apply
}
func (m *mockService) Delete(_ context.Context, _ *Event) error {
	m.calls = append(m.calls, "delete")
	return m.delete
}

func newMockEvent() *Event {
	return &Event{
		Id:   uuid.New(),
		Data: "some data",
	}
}

func TestAsActivity(t *testing.T) {
	m := &mockService{name: "mock"}
	// Reset calls before each action
	reset := func() { m.calls = nil }
	ctx := context.Background()
	for _, tc := range []struct {
		action   Action
		expected string
	}{
		{ValidateAction, "validate"},
		{ApplyAction, "apply"},
		{DeleteAction, "delete"},
	} {
		reset()
		evt := newMockEvent()
		activity := AsActivity(m, tc.action)
		if err := activity.Run(ctx, evt); err != nil {
			t.Errorf("activity for %s returned error: %v", tc.action, err)
		}
		if len(m.calls) != 1 || m.calls[0] != tc.expected {
			t.Errorf("expected %q to be called, got %v", tc.expected, m.calls)
		}
	}
}

func TestNewValidateWorkflow(t *testing.T) {
	m1 := &mockService{name: "svc1"}
	m2 := &mockService{name: "svc2"}
	wf := NewValidateWorkflow(m1, m2)
	if wf == nil {
		t.Fatalf("expected workflow, got nil")
	}
	// Reset calls and execute workflow
	m1.calls = nil
	m2.calls = nil
	evt := newMockEvent()
	if err := wf.Execute(context.Background(), evt); err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if len(m1.calls) != 1 || m1.calls[0] != "validate" {
		t.Errorf("expected m1 validate to be called, got %v", m1.calls)
	}
	if len(m2.calls) != 1 || m2.calls[0] != "validate" {
		t.Errorf("expected m2 validate to be called, got %v", m2.calls)
	}
}

func TestNewApplyWorkflow(t *testing.T) {
	m1 := &mockService{name: "svc1"}
	wf := NewApplyWorkflow(m1)
	if wf == nil {
		t.Fatalf("expected workflow, got nil")
	}
	m1.calls = nil
	evt := newMockEvent()
	if err := wf.Execute(context.Background(), evt); err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if len(m1.calls) != 1 || m1.calls[0] != "apply" {
		t.Errorf("expected m1 apply to be called, got %v", m1.calls)
	}
}

func TestNewDeleteWorkflow(t *testing.T) {
	m1 := &mockService{name: "svc1"}
	wf := NewDeleteWorkflow(m1)
	if wf == nil {
		t.Fatalf("expected workflow, got nil")
	}
	m1.calls = nil
	evt := newMockEvent()
	if err := wf.Execute(context.Background(), evt); err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if len(m1.calls) != 1 || m1.calls[0] != "delete" {
		t.Errorf("expected m1 delete to be called, got %v", m1.calls)
	}
}

func TestAsActivity_InvalidActionPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for invalid action")
		}
	}()
	_ = AsActivity(&mockService{name: "mock"}, Action("invalid"))
}

func TestWorkflowStepErrors(t *testing.T) {
	m := &mockService{name: "svc", validate: errors.New("fail validate"), apply: errors.New("fail apply"), delete: errors.New("fail delete")}
	wf := NewValidateWorkflow(m)
	evt := newMockEvent()
	err := wf.Execute(context.Background(), evt)
	if err == nil || !strings.Contains(err.Error(), "fail validate") {
		t.Errorf("expected fail validate, got %v", err)
	}
	wf = NewApplyWorkflow(m)
	err = wf.Execute(context.Background(), evt)
	if err == nil || !strings.Contains(err.Error(), "fail apply") {
		t.Errorf("expected fail apply, got %v", err)
	}
	wf = NewDeleteWorkflow(m)
	err = wf.Execute(context.Background(), evt)
	if err == nil || !strings.Contains(err.Error(), "fail delete") {
		t.Errorf("expected fail delete, got %v", err)
	}
}
