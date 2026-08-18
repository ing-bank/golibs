package tripperware

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/sony/gobreaker"
)

type Breaker struct {
	cfg *CircuitBreakerConfig

	breakerPoolLock sync.Mutex
	breakerPool     map[string]*gobreaker.CircuitBreaker
}

type CircuitBreakerConfig struct {
	Disabled  bool                `yaml:"disabled" json:"disabled"`
	GoBreaker *gobreaker.Settings `yaml:"-" json:"-"`
}

func (c *CircuitBreakerConfig) ApplyDefaults() {
	if c.GoBreaker == nil {
		c.GoBreaker = &gobreaker.Settings{
			MaxRequests: 100,
			Interval:    time.Minute,
			Timeout:     time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= 10
			},
			OnStateChange: func(string, gobreaker.State, gobreaker.State) {},
			IsSuccessful: func(err error) bool {
				return err == nil
			},
		}
	}
}

type CircuitBreakerOptions = config.Option[*CircuitBreakerConfig]

func NewBreakerForConfig(cfg CircuitBreakerConfig) (*Breaker, error) {
	if err := config.Configure(&cfg); err != nil {
		return nil, err
	}

	return &Breaker{
		cfg:             &cfg,
		breakerPoolLock: sync.Mutex{},
		breakerPool:     make(map[string]*gobreaker.CircuitBreaker),
	}, nil
}

func NewBreaker(opts ...CircuitBreakerOptions) (*Breaker, error) {
	cfg, err := config.New[CircuitBreakerConfig]()
	if err != nil {
		return nil, err
	}
	if err := config.ApplyOptions(cfg, opts...); err != nil {
		return nil, err
	}
	return NewBreakerForConfig(*cfg)
}

func WithCircuitBreakerSettings(settings *gobreaker.Settings) CircuitBreakerOptions {
	return func(b *CircuitBreakerConfig) error {
		b.GoBreaker = settings
		return nil
	}
}

func (b *Breaker) Tripperware() Tripperware {
	if b.cfg.Disabled {
		return EmptyTripperware()
	}
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {
			//logger := log.WithContext(ctx).WithField("func", utils.GetFuncName())

			u, err := url.Parse(request.URL.String())
			if err != nil {
				return &response.Data{Err: err}
			}

			breaker := b.GetGoBreaker(u.Host)
			//counter := breaker.Counts()

			//tripperware.PromBreakerRequestCounter.WithLabelValues(b.BreakerPool, breaker.State().String()).Set(float64(counter.Requests + 1)) // add one, because the breaker.Counts() is executed before the main loop
			//logger.WithFields(log.Fields{"breaker": breaker.Name(), "method": req.Method, "url": req.URL, "state": breaker.State().String()}).Debugln("Gobreaker status")

			resp, _ := breaker.Execute(func() (any, error) {
				return next(ctx, request), nil
			})
			return resp.(*response.Data)
		}
	}
}

func (b *Breaker) GetGoBreaker(name string) *gobreaker.CircuitBreaker {
	b.breakerPoolLock.Lock()
	defer b.breakerPoolLock.Unlock()

	breaker, ok := b.breakerPool[name]
	if !ok {
		breaker = gobreaker.NewCircuitBreaker(*b.cfg.GoBreaker)
		b.breakerPool[name] = breaker
	}

	return breaker
}
