package tripperware

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/http/response"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RateLimiter struct {
	cfg *RateLimiterConfig

	rateLimitLock sync.Mutex
	rateLimitPool map[string]*rate.Limiter
}

type RateLimiterOptions = config.Option[*RateLimiterConfig]

type RateLimiterConfig struct {
	Disabled        bool `yaml:"disabled" json:"disabled"`
	Endpoints       map[string]struct {
		Interval metav1.Duration `yaml:"interval" json:"interval"`
		Burst    int             `yaml:"burst" json:"burst"`
	} `yaml:"endpoints" json:"endpoints"`
	DefaultInterval metav1.Duration `yaml:"defaultInterval" json:"defaultInterval"`
	DefaultBurst    int             `yaml:"defaultBurst" json:"defaultBurst"`
}

func (r *RateLimiterConfig) ApplyDefaults() {
	if r.DefaultInterval.Duration == 0 {
		r.DefaultInterval = metav1.Duration{Duration: 50 * time.Millisecond}
	}
	if r.DefaultBurst == 0 {
		r.DefaultBurst = 10
	}
}

func NewRateLimiterForConfig(cfg RateLimiterConfig) (*RateLimiter, error) {
	if err := config.Configure(&cfg); err != nil {
		return nil, err
	}
	return &RateLimiter{
		cfg:           &cfg,
		rateLimitLock: sync.Mutex{},
		rateLimitPool: make(map[string]*rate.Limiter),
	}, nil
}

func NewRateLimiter(opts ...RateLimiterOptions) (*RateLimiter, error) {
	cfg, err := config.New[RateLimiterConfig]()
	if err != nil {
		return nil, err
	}
	if err := config.ApplyOptions(cfg, opts...); err != nil {
		return nil, err
	}
	return NewRateLimiterForConfig(*cfg)
}

func (r *RateLimiter) Tripperware() Tripperware {
	if r.cfg.Disabled {
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

	endpointDefault, ok := r.cfg.Endpoints[name]
	if !ok {
		endpointDefault.Burst = r.cfg.DefaultBurst
		endpointDefault.Interval = r.cfg.DefaultInterval
	}

	limiter = rate.NewLimiter(rate.Every(endpointDefault.Interval.Duration), endpointDefault.Burst)
	r.rateLimitPool[name] = limiter

	return limiter
}
