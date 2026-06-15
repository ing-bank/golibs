package healthcheck

import (
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
	"github.com/ing-bank/golibs/pkg/store/utilities/timed"
	"github.com/ing-bank/golibs/pkg/task/job"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultSyncPeriod = 10 * time.Second
	DefaultInterval   = 5 * time.Second
)

type Config struct {
	Jobs            []JobConfig     `json:"probes" yaml:"probes"`
	Interval        metav1.Duration `json:"interval" yaml:"interval"`
	CacheConfig     timed.Config    `json:"cache" yaml:"cache"`
	SystemInfo      bool            `json:"systemInfo" yaml:"systemInfo"`
	ComponentConfig *Component      `json:"component" yaml:"component"`
}

type JobConfig struct {
	job.Config    `json:",inline" yaml:",inline"`
	ProbeHandler  `json:",inline" yaml:",inline"`
	CustomHandler checks.Handler `json:",inline" yaml:",inline"`
	Endpoints     []Endpoint     `json:"endpoints,omitempty" yaml:"endpoints"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

// ApplyDefaultConfig applies the default values to the config.
func ApplyDefaultConfig(cfg *Config) {
	cfg.ApplyDefaults()
}

func (c *Config) ApplyDefaults() {
	if c.Interval.Duration <= 0 {
		c.Interval = metav1.Duration{Duration: DefaultInterval}
	}
	if c.CacheConfig.SyncPeriod.Duration <= 0 {
		c.CacheConfig.SyncPeriod.Duration = c.Interval.Duration + (5 * time.Second)
	}
	if c.CacheConfig.MaxAge.Duration <= 0 {
		// should be greater than the sync period to avoid cache misses
		c.CacheConfig.MaxAge.Duration = c.CacheConfig.SyncPeriod.Duration * 2
	}
}

func (c *Config) Validate() error {
	if c.Interval.Duration <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}
	if err := c.CacheConfig.Validate(); err != nil {
		return fmt.Errorf("invalid cache config: %w", err)
	}
	if c.Interval.Duration > c.CacheConfig.SyncPeriod.Duration {
		return fmt.Errorf("interval must be less than or equal to cache sync period")
	}
	for _, j := range c.Jobs {
		if err := j.Validate(); err != nil {
			return err
		}
	}
	return nil
}
