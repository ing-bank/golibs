package operator

import (
	"fmt"
	"math"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BackoffConfig controls non-blocking retry delays for failed workflows.
type BackoffConfig struct {
	// MinDelay is the first retry delay. Zero defaults to 1 second.
	MinDelay metav1.Duration `json:"minDelay"`
	// MaxDelay caps retry delays. Zero defaults to 5 minutes.
	MaxDelay metav1.Duration `json:"maxDelay"`
	// Factor multiplies subsequent delays. Zero defaults to 2; 1 is constant.
	Factor float64 `json:"factor,omitempty"`
}

// ApplyDefaults fills unset backoff fields.
func (c *BackoffConfig) ApplyDefaults() {
	if c.MinDelay.Duration == 0 {
		c.MinDelay.Duration = 1 * time.Second //nolint:mnd
	}
	if c.MaxDelay.Duration == 0 {
		c.MaxDelay.Duration = 5 * time.Minute //nolint:mnd
	}
	if c.Factor == 0 {
		c.Factor = 1.5 //nolint:mnd
	}
}

// Validate checks retry bounds and the exponential factor after defaults are applied.
func (c *BackoffConfig) Validate() error {
	if c.MinDelay.Duration <= 0 || c.MaxDelay.Duration < c.MinDelay.Duration {
		return fmt.Errorf("operator: backoff delays must satisfy 0 < minDelay <= maxDelay")
	}
	if c.Factor < 1 || math.IsNaN(c.Factor) || math.IsInf(c.Factor, 0) {
		return fmt.Errorf("operator: backoff factor must be finite and at least 1")
	}
	return nil
}

type retryState struct {
	uid        types.UID
	generation int64
	deleting   bool
	delay      time.Duration
}

type retryBackoff struct {
	cfg   BackoffConfig
	items sync.Map // NamespacedName -> retryState; reconciles are serialized per key by controller-runtime.
}

func (b *retryBackoff) next(item client.Object) ctrl.Result {
	key := client.ObjectKeyFromObject(item)
	state := retryState{
		uid:        item.GetUID(),
		generation: item.GetGeneration(),
		deleting:   item.GetDeletionTimestamp() != nil && !item.GetDeletionTimestamp().IsZero(),
		delay:      b.cfg.MinDelay.Duration,
	}
	if value, ok := b.items.Load(key); ok {
		previous := value.(retryState)
		if previous.uid == state.uid && previous.generation == state.generation && previous.deleting == state.deleting {
			// Cap before converting to Duration to avoid overflow at large factors.
			delay := float64(previous.delay) * b.cfg.Factor
			if delay >= float64(b.cfg.MaxDelay.Duration) {
				state.delay = b.cfg.MaxDelay.Duration
			} else {
				state.delay = max(previous.delay, time.Duration(delay))
			}
		}
	}
	b.items.Store(key, state)
	return ctrl.Result{RequeueAfter: state.delay}
}

func (b *retryBackoff) forget(key types.NamespacedName) ctrl.Result {
	b.items.Delete(key)
	return ctrl.Result{}
}
