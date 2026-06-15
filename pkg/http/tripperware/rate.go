package tripperware

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/http/response"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	*RateLimitSettings

	rateLimitLock sync.Mutex
	rateLimitPool map[string]*rate.Limiter
}

type RateLimiterOptions func(*RateLimiter) error

type RateLimitSettings struct {
	Endpoints map[string]struct {
		Interval time.Duration
		Burst    int
	}
	DefaultInterval time.Duration
	DefaultBurst    int
}

func NewRateLimiter(opts ...RateLimiterOptions) *RateLimiter {
	rateLimit := &RateLimiter{
		RateLimitSettings: &RateLimitSettings{
			DefaultInterval: 50 * time.Millisecond,
			DefaultBurst:    10,
		},
		rateLimitLock: sync.Mutex{},
		rateLimitPool: make(map[string]*rate.Limiter),
	}
	for _, opt := range opts {
		_ = opt(rateLimit)
	}
	return rateLimit
}

func WithRateLimitSettings(settings *RateLimitSettings) RateLimiterOptions {
	return func(r *RateLimiter) error {
		r.RateLimitSettings = settings
		return nil
	}
}

func (r *RateLimiter) Tripperware() Tripperware {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {

			u, err := url.Parse(request.URL.String())
			if err != nil {
				return &response.Data{Err: err}
			}

			limiter := r.GetRateLimitWithSettings(u.Host)

			err = limiter.Wait(ctx)
			if err != nil {
				return &response.Data{Err: err}
			}
			return next(ctx, request)
		}
	}
}

func (r *RateLimiter) GetRateLimitWithSettings(name string) *rate.Limiter {
	r.rateLimitLock.Lock()
	defer r.rateLimitLock.Unlock()

	limiter, ok := r.rateLimitPool[name]
	if ok {
		return limiter
	}

	endpointDefault, ok := r.Endpoints[name]
	if !ok {
		endpointDefault.Burst = r.DefaultBurst
		endpointDefault.Interval = r.DefaultInterval
	}

	limiter = rate.NewLimiter(rate.Every(endpointDefault.Interval), endpointDefault.Burst)
	r.rateLimitPool[name] = limiter

	return limiter
}
