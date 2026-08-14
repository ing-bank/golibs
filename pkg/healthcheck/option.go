package healthcheck

import (
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
)

type Option = config.Option[*HealthCheck]

// WithNewChecks create new checks
func WithNewChecks(checks ...JobConfig) config.Option[*HealthCheck] {
	return func(h *HealthCheck) error {
		// reset existing checks
		h.Checks = nil
		// add new checks
		if err := h.Add(checks...); err != nil {
			return fmt.Errorf("could not register check: %w", err)
		}
		return nil
	}
}

// WithAddChecks adds checks to newly instantiated health-container
func WithAddChecks(checks ...JobConfig) config.Option[*HealthCheck] {
	return func(h *HealthCheck) error {
		if err := h.Add(checks...); err != nil {
			return fmt.Errorf("could not register check: %w", err)
		}
		return nil
	}
}

// WithComponent sets the component description of the component to which this check refer
func WithComponent(component *Component) config.Option[*HealthCheck] {
	return func(h *HealthCheck) error {
		h.Component = component
		return nil
	}
}

// WithSystemInfo enables the option to return system information about the go process.
func WithSystemInfo() config.Option[*HealthCheck] {
	return func(h *HealthCheck) error {
		h.systemInfoEnabled = true
		return nil
	}
}

// WithInterval sets the interval at which the checks are executed
func WithInterval(interval time.Duration) config.Option[*HealthCheck] {
	return func(h *HealthCheck) error {
		if interval <= 0 {
			return fmt.Errorf("interval must be greater than zero")
		}
		h.interval = interval
		return nil
	}
}
