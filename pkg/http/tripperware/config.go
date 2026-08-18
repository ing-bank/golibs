package tripperware

import "github.com/ing-bank/golibs/pkg/config"

// For posterity: Tripperware does not have a runtime client struct at the moment, it just returns a function.
// Therefore, it is not meaninful to have a config.Option[*Tripperware] type, as there is no client
// struct to apply the options to. It's possible to have options that act on the config, but the benefit
// would be very small. Instead, lets skip the config options for now. This allows future extensions to this
// package to define tripperware options on a client when that is introduced in a backwards compatible fashion.
// type Option = config.Option[*Config/Tripperware]

type Config struct {
	Breaker     *CircuitBreakerConfig `yaml:"breaker" json:"breaker"`
	Logging     *LoggingConfig        `yaml:"logging" json:"logging"`
	Metrics     *MetricsConfig        `yaml:"metrics" json:"metrics"`
	RateLimiter *RateLimiterConfig    `yaml:"rateLimiter" json:"rateLimiter"`
	Retrier     *RetrierConfig        `yaml:"retrier" json:"retrier"`
}

func (c *Config) ApplyDefaults() {
	if c.Breaker == nil {
		c.Breaker = &CircuitBreakerConfig{Disabled: false}
	}
	if c.Logging == nil {
		c.Logging = &LoggingConfig{Disabled: false}
	}
	if c.Metrics == nil {
		c.Metrics = &MetricsConfig{Disabled: false}
	}
	if c.RateLimiter == nil {
		c.RateLimiter = &RateLimiterConfig{Disabled: false}
	}
	if c.Retrier == nil {
		c.Retrier = &RetrierConfig{Disabled: false}
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

func New() (Tripperware, error) {
	return NewForConfig(Config{})
}

func NewOrDie() Tripperware {
	tw, err := New()
	if err != nil {
		panic(err)
	}
	return tw
}
