package tripperware

import "github.com/ing-bank/golibs/pkg/config"

type Option = config.Option[*Config]

type Config struct {
	Breaker     *CircuitBreakerConfig `yaml:"breaker" json:"breaker"`
	Logging     *LoggingConfig        `yaml:"logging" json:"logging"`
	Metrics     *MetricsConfig        `yaml:"metrics" json:"metrics"`
	RateLimiter *RateLimiterConfig    `yaml:"rateLimiter" json:"rateLimiter"`
	Retrier     *RetrierConfig        `yaml:"retrier" json:"retrier"`
}

func (c *Config) ApplyDefaults() {
	if c.Breaker == nil {
		c.Breaker = &CircuitBreakerConfig{Enabled: true}
	}
	if c.Logging == nil {
		c.Logging = &LoggingConfig{}
	}
	if c.Metrics == nil {
		c.Metrics = &MetricsConfig{Enabled: true}
	}
	if c.RateLimiter == nil {
		c.RateLimiter = &RateLimiterConfig{Enabled: true}
	}
	if c.Retrier == nil {
		c.Retrier = &RetrierConfig{Enabled: true}
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
	logger, err := NewLoggingForConfig(*cfg.Logging)
	if err != nil {
		return nil, err
	}
	breaker, err := NewBreakerForConfig(*cfg.Breaker)
	if err != nil {
		return nil, err
	}
	limiter, err := NewRateLimiterForConfig(*cfg.RateLimiter)
	if err != nil {
		return nil, err
	}
	metrics, err := NewMetricsForConfig(*cfg.Metrics)
	if err != nil {
		return nil, err
	}

	return Chain(
		retrier.Tripperware(),
		logger,
		breaker.Tripperware(),
		limiter.Tripperware(),
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
