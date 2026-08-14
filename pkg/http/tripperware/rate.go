package tripperware

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/http/response"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RateLimiter struct {
	*RateLimitSettings

	rateLimitLock sync.Mutex
	rateLimitPool map[string]*rate.Limiter
}

type RateLimiterOptions func(*RateLimiter) error

type RateLimitSettings struct {
	Enabled   bool `yaml:"enabled" json:"enabled"`
	Endpoints map[string]struct {
		Interval metav1.Duration `yaml:"interval" json:"interval"`
		Burst    int             `yaml:"burst" json:"burst"`
	}
	DefaultInterval metav1.Duration `yaml:"defaultInterval" json:"defaultInterval"`
	DefaultBurst    int             `yaml:"defaultBurst" json:"defaultBurst"`
}

func (r *RateLimitSettings) ApplyDefaults() {
	if r.DefaultInterval.Duration == 0 {
		r.DefaultInterval = metav1.Duration{Duration: 50 * time.Millisecond}
	}
	if r.DefaultBurst == 0 {
		r.DefaultBurst = 10
	}
}

func NewRateLimiterForConfig(cfg RateLimitSettings) *RateLimiter {
	cfg.ApplyDefaults()
	return &RateLimiter{
		RateLimitSettings: &cfg,
		rateLimitLock:     sync.Mutex{},
		rateLimitPool:     make(map[string]*rate.Limiter),
	}
}

func NewRateLimiter(opts ...RateLimiterOptions) *RateLimiter {
	rateLimit := NewRateLimiterForConfig(RateLimitSettings{Enabled: true})
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
	if !r.Enabled {
		return EmptyTripperware()
	}
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

	limiter = rate.NewLimiter(rate.Every(endpointDefault.Interval.Duration), endpointDefault.Burst)
	r.rateLimitPool[name] = limiter

	return limiter
}
