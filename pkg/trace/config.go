package trace

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

const (
	DefaultJaegerEndpoint = "http://localhost:14268/api/traces"
	DefaultEnvironment    = "development"

	FlagTraceEndpoint       = "trace-endpoint"
	FlagTraceServiceName    = "trace-service-name"
	FlagTraceServiceVersion = "trace-service-version"
	FlagTraceEnvironment    = "trace-environment"
	FlagTraceEnabled        = "trace-enabled"
	FlagTraceSkipPaths      = "trace-skip-paths"
)

var DefaultSkipPaths = []string{"/metrics", "/healthz", "/readyz", "/swagger"}

// Config represents the provider configuration of OpenTelemetry
type Config struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	JaegerEndpoint string   `json:"jaegerEndpoint,omitempty" yaml:"jaegerEndpoint"`
	ServiceName    string   `json:"serviceName" yaml:"serviceName"`
	ServiceVersion string   `json:"serviceVersion" yaml:"serviceVersion"`
	Environment    string   `json:"environment" yaml:"environment"`
	SkipPaths      []string `json:"skipPaths" yaml:"skipPaths"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.ServiceName == "" {
		hostname, err := os.Hostname()
		if err == nil {
			cfg.ServiceName = hostname
		}
		// if we can't get the hostname, fallback to "unknown"
		if cfg.ServiceName == "" {
			cfg.ServiceName = "unknown"
		}
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "0.0.0"
	}
	if cfg.Environment == "" {
		cfg.Environment = DefaultEnvironment
	}
	if len(cfg.SkipPaths) == 0 {
		cfg.SkipPaths = DefaultSkipPaths
	}
	if cfg.JaegerEndpoint == "" {
		cfg.JaegerEndpoint = DefaultJaegerEndpoint
	}
}

func (c *Config) ApplyDefaults() {
	ApplyDefaults(c)
}

func init() {
	if os.Getenv("PFLAGS_TRACE_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()

	flags.String(FlagTraceEndpoint, c.JaegerEndpoint, "Jaeger collector endpoint")
	flags.String(FlagTraceServiceName, c.ServiceName, "Service name")
	flags.String(FlagTraceServiceVersion, c.ServiceVersion, "Service version")
	flags.String(FlagTraceEnvironment, c.Environment, "Service environment (e.g., production, staging, development)")
	flags.Bool(FlagTraceEnabled, c.Enabled, "Enable tracing")
	flags.StringSlice(FlagTraceSkipPaths, c.SkipPaths, "Comma-separated list of paths to skip for tracing")
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if fs.Changed(FlagTraceEndpoint) {
		if c.JaegerEndpoint, err = fs.GetString(FlagTraceEndpoint); err != nil {
			return fmt.Errorf("jaeger endpoint: %w", err)
		}
	}
	if fs.Changed(FlagTraceServiceName) {
		if c.ServiceName, err = fs.GetString(FlagTraceServiceName); err != nil {
			return fmt.Errorf("service name: %w", err)
		}
	}
	if fs.Changed(FlagTraceServiceVersion) {
		if c.ServiceVersion, err = fs.GetString(FlagTraceServiceVersion); err != nil {
			return fmt.Errorf("service version: %w", err)
		}
	}
	if fs.Changed(FlagTraceEnvironment) {
		if c.Environment, err = fs.GetString(FlagTraceEnvironment); err != nil {
			return fmt.Errorf("environment: %w", err)
		}
	}
	if fs.Changed(FlagTraceEnabled) {
		if c.Enabled, err = fs.GetBool(FlagTraceEnabled); err != nil {
			return fmt.Errorf("enabled: %w", err)
		}
	}
	if fs.Changed(FlagTraceSkipPaths) {
		if c.SkipPaths, err = fs.GetStringSlice(FlagTraceSkipPaths); err != nil {
			return fmt.Errorf("skip paths: %w", err)
		}
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Enabled {
		if c.JaegerEndpoint == "" {
			return fmt.Errorf("jaeger endpoint is required when tracing is enabled")
		}
		if c.ServiceName == "" {
			return fmt.Errorf("service name is required when tracing is enabled")
		}
		if c.ServiceVersion == "" {
			return fmt.Errorf("service version is required when tracing is enabled")
		}
		if c.Environment == "" {
			return fmt.Errorf("environment is required when tracing is enabled")
		}
	}
	return nil
}
