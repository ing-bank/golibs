package job

import (
	"context"
	"errors"
	"testing"
	"time"
)

func exampleJob(name string, mayFail bool) Job {
	return NewOrDie(name, "desc "+name, 2*time.Second, mayFail, func(ctx context.Context) error { return nil })
}

func TestJobResult_Methods(t *testing.T) {
	t.Parallel()
	job := exampleJob("jobX", false)
	result := Run(t.Context(), job)
	if result.Name != job.Config.Name {
		t.Errorf("JobResult name mismatch")
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
	if !result.FinishedAt.After(result.StartedAt) && !result.FinishedAt.Equal(result.StartedAt) {
		t.Errorf("FinishedAt should be after StartedAt")
	}
	if !result.IsSuccessful() {
		t.Errorf("IsSuccessful should be true")
	}
	if result.HasErrors() {
		t.Errorf("HasErrors should be false")
	}
	if result.IsRunning() {
		t.Errorf("IsRunning should be false after completion")
	}
}

func TestJobResult_ErrorHandling(t *testing.T) {
	t.Parallel()
	job := NewOrDie("failJob", "desc failJob", 2*time.Second, false, func(ctx context.Context) error {
		return errors.New("fail error")
	})
	result := Run(t.Context(), job)
	if result.Error == nil || result.Error.Error() != "fail error" {
		t.Errorf("expected error 'fail error', got %v", result.Error)
	}
	if result.IsSuccessful() {
		t.Errorf("IsSuccessful should be false")
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors should be true")
	}
}

func TestJobResult_Timeout(t *testing.T) {
	t.Parallel()
	job := NewOrDie("timeoutJob", "desc timeoutJob", 500*time.Millisecond, false, func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		return nil
	})
	result := Run(t.Context(), job)
	if result.Error == nil || result.Error.Error() != "job timed out" {
		t.Errorf("expected timeout error, got %v", result.Error)
	}
	if result.IsSuccessful() {
		t.Errorf("IsSuccessful should be false")
	}
	if !result.HasErrors() {
		t.Errorf("HasErrors should be true")
	}
}

func TestJobResult_MayFail(t *testing.T) {
	t.Parallel()
	job := NewOrDie("mayFailJob", "desc mayFailJob", 2*time.Second, true, func(ctx context.Context) error {
		return errors.New("fail error")
	})
	result := Run(t.Context(), job)
	if result.Error == nil || result.Error.Error() != "fail error" {
		t.Errorf("expected error 'fail error', got %v", result.Error)
	}
	if result.GetAvailability() != StatusPartiallyAvailable {
		t.Errorf("expected StatusPartiallyAvailable, got %v", result.GetAvailability())
	}
}

func TestJobResult_GetAvailability(t *testing.T) {
	t.Parallel()
	successJob := NewOrDie("success", "desc", 1*time.Second, false, func(ctx context.Context) error { return nil })
	failJob := NewOrDie("fail", "desc", 1*time.Second, false, func(ctx context.Context) error { return errors.New("fail") })
	mayFailJob := NewOrDie("mayfail", "desc", 1*time.Second, true, func(ctx context.Context) error { return errors.New("fail") })

	successResult := Run(t.Context(), successJob)
	failResult := Run(t.Context(), failJob)
	mayFailResult := Run(t.Context(), mayFailJob)

	if successResult.GetAvailability() != StatusOK {
		t.Errorf("expected StatusOK, got %v", successResult.GetAvailability())
	}
	if failResult.GetAvailability() != StatusUnavailable {
		t.Errorf("expected StatusUnavailable, got %v", failResult.GetAvailability())
	}
	if mayFailResult.GetAvailability() != StatusPartiallyAvailable {
		t.Errorf("expected StatusPartiallyAvailable, got %v", mayFailResult.GetAvailability())
	}
}

func TestJobResult_MarkAsStartedAndCompleted(t *testing.T) {
	t.Parallel()
	job := exampleJob("jobMark", false)
	result := ToJobResult(job)
	if !result.StartedAt.IsZero() || !result.FinishedAt.IsZero() {
		t.Errorf("expected zero times before marking")
	}
	result.MarkAsStarted()
	if result.StartedAt.IsZero() {
		t.Errorf("expected StartedAt to be set")
	}
	result.MarkAsCompleted()
	if result.FinishedAt.IsZero() {
		t.Errorf("expected FinishedAt to be set")
	}
}

func TestJob_Validate(t *testing.T) {
	if err := (&Config{}).Validate(); err == nil {
		t.Errorf("expected validation error for empty config")
	}

	if err := (&Job{
		Config: Config{Name: "foo"},
	}).Validate(); err == nil {
		t.Errorf("expected validation error for empty job")
	}
}
