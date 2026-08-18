package http

import (
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/http/tripperware"
	"github.com/ing-bank/golibs/pkg/tlsclient"
)

// Config holds configuration for the HTTP client.
type Config struct {
	TLS            *tlsclient.Config  `json:"tls" yaml:"tls"`
	DefaultHeaders map[string]string  `json:"defaultHeaders" yaml:"defaultHeaders"`
	Tripperware    tripperware.Config `json:"tripperware" yaml:"tripperware"`
	FollowRedirect bool               `json:"followRedirect" yaml:"followRedirect"`
}

// DefaultConfig is a function that returns a *Config struct with default values applied.
var DefaultConfig = config.DefaultConfig[Config]() // Calls ApplyDefaults on the new Config struct

// ApplyDefaults sets default values for the Config struct fields if they are not provided.
func (c *Config) ApplyDefaults() {
	if c.TLS != nil {
		c.TLS.ApplyDefaults()
	}
	c.Tripperware.ApplyDefaults()
	c.DefaultHeaders = make(map[string]string)
}

// Validate checks if the Config struct has valid values.
func (c *Config) Validate() error {
	return nil
}
