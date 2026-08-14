package tripperware

import "github.com/ing-bank/golibs/pkg/config"

type Option = config.Option[*Config]

type Config struct {
	Breaker     *BreakerConfig     `yaml:"breaker" json:"breaker"`
	Logging     *LoggingOptions    `yaml:"logging" json:"logging"`
	Metrics     *MetricsConfig     `yaml:"metrics" json:"metrics"`
	RateLimiter *RateLimitSettings `yaml:"rateLimiter" json:"rateLimiter"`
	Retrier     *RetrierConfig     `yaml:"retrier" json:"retrier"`
}

func WithDefaultConfig[T any](provided *T, def T) T {
	if provided == nil {
		return def
	}
	return *provided
}

func (c *Config) ApplyDefaults() {
	if c.Breaker == nil {
		c.Breaker = &BreakerConfig{}
	}
	if c.Logging == nil {
		c.Logging = &LoggingOptions{}
	}
	if c.Metrics == nil {
		c.Metrics = &MetricsConfig{}
	}
	if c.RateLimiter == nil {
		c.RateLimiter = &RateLimitSettings{}
	}
	if c.Retrier == nil {
		c.Retrier = DefaultRetrierConfig()
	}
}

func NewForConfig(cfg Config) (Tripperware, error) {
	if err := config.Configure(&cfg); err != nil {
		return nil, err
	}

	retrier, err := NewRetrierForConfig(*cfg.Retrier)
	if err != nil {
		return nil, err
	}

	metrics, err := NewMetricsForConfig(*cfg.Metrics)
	if err != nil {
		return nil, err
	}

	return Chain(
		retrier.Tripperware(),
		Logging(WithDefaultConfig(cfg.Logging, LoggingOptions{Enabled: true})),
		NewBreakerForConfig(WithDefaultConfig(cfg.Breaker, BreakerConfig{Enabled: true})).Tripperware(),
		NewRateLimiterForConfig(WithDefaultConfig(cfg.RateLimiter, RateLimitSettings{Enabled: true})).Tripperware(),
		metrics,
	), nil
}

func New(opts ...Option) (Tripperware, error) {
	cfg, err := config.New[Config](opts...)
	if err != nil {
		return nil, err
	}
	return NewForConfig(*cfg)
}

func NewOrDie(opts ...Option) Tripperware {
	cfg, err := config.New[Config](opts...)
	if err != nil {
		panic(err)
	}
	tw, err := NewForConfig(*cfg)
	if err != nil {
		panic(err)
	}
	return tw
}
