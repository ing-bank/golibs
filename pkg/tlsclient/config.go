package tlsclient

import (
	"github.com/ing-bank/golibs/pkg/tlsutils"
)

type Config struct {
	tlsutils.Config    `json:",inline" yaml:",inline"`
	InsecureSkipVerify bool `json:"insecureSkipVerify"`
}

func NewConfig(cert, key string, insecureSkipVerify bool, cacerts ...string) *Config {
	return &Config{
		Config:             *tlsutils.NewConfig(cert, key, cacerts...),
		InsecureSkipVerify: insecureSkipVerify,
	}
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func (c *Config) ApplyDefaults() {
	c.Config.ApplyDefaults()
}
