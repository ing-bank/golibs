package timed

import (
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	RefreshAgeOnRead bool            `json:"refreshAgeOnRead" yaml:"refreshAgeOnRead"` // Whether to refresh the age of an item on read
	SyncPeriod       metav1.Duration `json:"syncPeriod" yaml:"syncPeriod"`             // How often to run maintenance jobs
	MaxAge           metav1.Duration `json:"maxAge" yaml:"maxAge"`                     // Maximum age of items in seconds
}

func DefaultConfig() *Config {
	return &Config{
		SyncPeriod: metav1.Duration{Duration: time.Hour}, // Default sync period
		MaxAge:     metav1.Duration{Duration: time.Hour}, // Default max age of items
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("options cannot be nil")
	}
	if c.MaxAge.Duration <= 0 {
		return errors.New("MaxAgeSec must be greater than 0")
	}
	if c.SyncPeriod.Duration <= 0 {
		return errors.New("SyncPeriod must be greater than 0")
	}
	return nil
}
