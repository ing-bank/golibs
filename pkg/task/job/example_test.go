package job

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func ExampleRunAny() {
	jobs := []Job{
		NewOrDie("Job1", "Always succeeds", 2*time.Second, false, func(ctx context.Context) error {
			return nil
		}),
		NewOrDie("Job2", "Always fails", 2*time.Second, false, func(ctx context.Context) error {
			return errors.New("failure")
		}),
		NewOrDie("Job3", "Timeouts", 1*time.Second, false, func(ctx context.Context) error {
			time.Sleep(2 * time.Second)
			return nil
		}),
	}

	results := RunAny(context.Background(), jobs)

	for _, result := range results {
		fmt.Printf("Job=%s, Description=%s, Timeout=%v, Error=%v, State=%s\n", result.Name, result.Description, result.Timeout, result.Error, result.State)
	}
	// Output:
	// Job=Job1, Description=Always succeeds, Timeout=2s, Error=<nil>, State=SUCCESS
	// Job=Job2, Description=Always fails, Timeout=2s, Error=failure, State=FAILED
	// Job=Job3, Description=Timeouts, Timeout=1s, Error=job timed out, State=FAILED
}

func ExampleRun() {
	job, _ := New("SingleJob", "Succeeds", 2*time.Second, false, func(ctx context.Context) error {
		return nil
	})

	result := Run(context.Background(), job)

	fmt.Printf("Job=%s, Description=%s, Timeout=%v, Error=%v, State=%s\n", result.Name, result.Description, result.Timeout, result.Error, result.State)
	// Output:
	// Job=SingleJob, Description=Succeeds, Timeout=2s, Error=<nil>, State=SUCCESS
}
