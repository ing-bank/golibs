package tripperware

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/sony/gobreaker"
)

type Breaker struct {
	*CircuitBreakerSettings

	breakerPoolLock sync.Mutex
	breakerPool     map[string]*gobreaker.CircuitBreaker
}

type CircuitBreakerOptions func(*Breaker) error

type CircuitBreakerSettings struct {
	BreakerPool string // TODO: broken by middleware implementation? Overriden by host var
	GoBreaker   gobreaker.Settings
}

func NewBreaker(opts ...CircuitBreakerOptions) *Breaker {
	breaker := &Breaker{
		CircuitBreakerSettings: &CircuitBreakerSettings{
			GoBreaker: gobreaker.Settings{
				// Name:        "",
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
			},
		},
		breakerPoolLock: sync.Mutex{},
		breakerPool:     make(map[string]*gobreaker.CircuitBreaker),
	}

	for _, opt := range opts {
		_ = opt(breaker)
	}

	return breaker

}

func WithCircuitBreakerSettings(settings *CircuitBreakerSettings) CircuitBreakerOptions {
	return func(b *Breaker) error {
		b.CircuitBreakerSettings = settings
		return nil
	}
}

func (b *Breaker) Tripperware() Tripperware {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {
			//logger := log.WithContext(ctx).WithField("func", utils.GetFuncName())

			u, err := url.Parse(request.URL.String())
			if err != nil {
				return &response.Data{Err: err}
			}
			// override breaker name with host var
			b.breakerPoolLock.Lock()
			b.BreakerPool = u.Host
			b.breakerPoolLock.Unlock()

			breaker := b.GetBreakerWithSettings()
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

func (b *Breaker) GetBreakerWithSettings() *gobreaker.CircuitBreaker {
	b.breakerPoolLock.Lock()
	defer b.breakerPoolLock.Unlock()

	breaker, ok := b.breakerPool[b.BreakerPool]
	if !ok {
		breaker = gobreaker.NewCircuitBreaker(b.GoBreaker)
		b.breakerPool[b.BreakerPool] = breaker
	}

	return breaker
}
