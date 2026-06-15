package graceful

import "time"

// RunAllOptions contains the configuration for running background tasks
type RunAllOptions struct {
	FailFast        bool
	ShutdownTimeout time.Duration
}

type RunOptions struct {
	ShutdownTimeout time.Duration
}

var FailFast = DefaultRunAllOptions()

// DefaultRunAllOptions returns the default options for running background tasks
func DefaultRunAllOptions() RunAllOptions {
	return RunAllOptions{
		FailFast:        DefaultFailFast,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// NewRunOptions creates a new RunAllOptions struct with the given parameters
func NewRunOptions(shutdownTimeout time.Duration) RunOptions {
	return RunOptions{
		ShutdownTimeout: shutdownTimeout,
	}
}

// NewRunAllOptions creates a new RunAllOptions struct with the given parameters
func NewRunAllOptions(failFast bool, shutdownTimeout time.Duration) RunAllOptions {
	return RunAllOptions{
		FailFast:        failFast,
		ShutdownTimeout: shutdownTimeout,
	}
}
