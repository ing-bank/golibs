package status

import (
	"context"
	"time"

	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/orchestration/audit"
	"github.com/ing-bank/golibs/pkg/trace"
)

type Status struct {
	Succeeded bool   `json:"succeeded"`
	State     string `json:"state"`
	TraceID   string `json:"traceID"`

	CreatedAt  audit.Audit `json:"createdAt" binding:"required"`
	UpdatedAt  audit.Audit `json:"updatedAt,omitempty"`
	Conditions []Condition `json:"conditions,omitempty"`
}

type Condition struct {
	State     string     `json:"state"`
	Message   string     `json:"message,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
}

func NewStatus(ctx context.Context, createdAt, updatedAt audit.Audit, initialState string) Status {
	return Status{
		Succeeded:  false,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		State:      initialState,
		TraceID:    trace.GetTraceIDFromContext(ctx).String(),
		Conditions: []Condition{NewCondition(initialState)},
	}
}

// Update the Status object of the request, taking a machine parseable state name
// and an optional message. The new Condition is also added to the list of Condition.
func (s *Status) Update(state string, isSucceeded bool, optionalMsg ...string) {
	msg := opt.Opt("", optionalMsg)

	if s.State == state && msg == "" {
		return // Nothing to do, state already set
	}

	if s.State == state && s.LastStatusMsg() == msg {
		s.Conditions[0].LastSeen = new(time.Now())
		return
	}

	s.Succeeded = isSucceeded
	s.State = state
	s.Conditions = append([]Condition{NewCondition(state, msg)}, s.Conditions...)
}

// SwitchState updates the state if, and only if, the current state does not match the provided state
func (s *Status) SwitchState(state string, msg string) {
	if s.State == state {
		return // Nothing to do, state already set
	}

	s.Update(state, s.Succeeded, msg)
}

func (s *Status) UpdateProgress(msg string) {
	s.Update(s.State, s.Succeeded, msg)
}

func (s *Status) LastStatusMsg() string {
	if len(s.Conditions) == 0 {
		return ""
	}
	return s.Conditions[0].Message
}

func NewCondition(state string, optMsg ...string) Condition {
	msg := opt.Opt("", optMsg)
	return Condition{
		State:     state,
		Message:   msg,
		Timestamp: time.Now(),
	}
}
