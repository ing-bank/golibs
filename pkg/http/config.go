package http

import (
	"github.com/ing-bank/golibs/pkg/tlsclient"
)

// Config holds configuration for the HTTP client.
type Config struct {
	TLS            tlsclient.Config  `json:"tls" yaml:"tls"`
	DefaultHeaders map[string]string `json:"defaultHeaders" yaml:"defaultHeaders"`
}

// DefaultConfig returns a Config struct with default values applied.
func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

// ApplyDefaults sets default values for the Config struct fields if they are not provided.
func (c *Config) ApplyDefaults() {
	ApplyDefaults(c)
}

// ApplyDefaults sets default values for the Config struct fields if they are not provided.
func ApplyDefaults(cfg *Config) *Config {
	cfg.TLS.ApplyDefaults()
	if cfg.DefaultHeaders == nil {
		cfg.DefaultHeaders = make(map[string]string)
	}
	return cfg
}

// Validate checks if the Config struct has valid values.
func (c *Config) Validate() error {
	return nil
}
