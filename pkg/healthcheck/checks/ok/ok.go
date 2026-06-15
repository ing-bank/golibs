package ok

import (
	"context"

	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
)

var _ checks.Handler = (*Config)(nil)

type Config struct {
}

func New() *Config {
	return &Config{}
}

func (o *Config) Check(ctx context.Context) error {
	return nil
}
