package operator

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/yaml"
)

func TestRetryBackoffDelays(t *testing.T) {
	tests := []struct {
		name   string
		cfg    BackoffConfig
		delays []time.Duration
	}{
		{"defaults", BackoffConfig{}, []time.Duration{time.Second, 1500 * time.Millisecond, 2250 * time.Millisecond}},
		{
			"factor and cap",
			BackoffConfig{MinDelay: metav1.Duration{Duration: time.Second}, MaxDelay: metav1.Duration{Duration: 5 * time.Second}, Factor: 3},
			[]time.Duration{time.Second, 3 * time.Second, 5 * time.Second, 5 * time.Second},
		},
		{
			"fractional factor",
			BackoffConfig{MinDelay: metav1.Duration{Duration: time.Second}, Factor: 1.5},
			[]time.Duration{time.Second, 1500 * time.Millisecond, 2250 * time.Millisecond},
		},
		{"constant", BackoffConfig{Factor: 1}, []time.Duration{time.Second, time.Second}},
		{
			"overflow",
			BackoffConfig{Factor: math.MaxFloat64, MaxDelay: metav1.Duration{Duration: time.Duration(math.MaxInt64)}},
			[]time.Duration{time.Second, time.Duration(math.MaxInt64), time.Duration(math.MaxInt64)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ApplyDefaults()
			require.NoError(t, tt.cfg.Validate())
			backoff := retryBackoff{cfg: tt.cfg}
			item := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1}}
			for _, delay := range tt.delays {
				require.Equal(t, ctrl.Result{RequeueAfter: delay}, backoff.next(item))
			}
		})
	}
}

func TestRetryBackoffIdentityAndReset(t *testing.T) {
	cfg := BackoffConfig{}
	cfg.ApplyDefaults()
	backoff := retryBackoff{cfg: cfg}
	item := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 1, UID: "original"}}
	require.Equal(t, cfg.MinDelay.Duration, backoff.next(item).RequeueAfter)

	for _, change := range []struct {
		name string
		fn   func()
	}{
		{"namespace", func() { item.Namespace = "other" }},
		{"name", func() { item.Name = "other" }},
		{"generation", func() { item.Generation++ }},
		{"recreation", func() { item.UID = "replacement" }},
		{"deletion", func() { now := metav1.Now(); item.DeletionTimestamp = &now }},
		{"forget", func() { require.Equal(t, ctrl.Result{}, backoff.forget(client.ObjectKeyFromObject(item))) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			change.fn()
			require.Equal(t, cfg.MinDelay.Duration, backoff.next(item).RequeueAfter)
			require.Equal(t, time.Duration(float64(cfg.MinDelay.Duration)*cfg.Factor), backoff.next(item).RequeueAfter)
		})
	}
	// A generation change replaces the entry rather than retaining old generations.
	require.Equal(t, 3, backoffSize(&backoff))
}

func TestRetryBackoffConcurrentObjects(t *testing.T) {
	cfg := BackoffConfig{}
	cfg.ApplyDefaults()
	backoff := retryBackoff{cfg: cfg}
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Go(func() {
			item := &testObj{ObjectMeta: metav1.ObjectMeta{Name: string(rune('a' + i))}}
			for range 10 {
				backoff.next(item)
			}
			backoff.forget(client.ObjectKeyFromObject(item))
		})
	}
	wg.Wait()
	require.Zero(t, backoffSize(&backoff))
}

func backoffSize(b *retryBackoff) (count int) {
	b.items.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func TestRetryConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*BackoffConfig)
	}{
		{"negative minimum", func(c *BackoffConfig) { c.MinDelay.Duration = -time.Second }},
		{"negative maximum", func(c *BackoffConfig) { c.MaxDelay.Duration = -time.Second }},
		{"inverted bounds", func(c *BackoffConfig) { c.MaxDelay.Duration = time.Millisecond }},
		{"decreasing factor", func(c *BackoffConfig) { c.Factor = 0.5 }},
		{"negative factor", func(c *BackoffConfig) { c.Factor = -2 }},
		{"NaN factor", func(c *BackoffConfig) { c.Factor = math.NaN() }},
		{"infinite factor", func(c *BackoffConfig) { c.Factor = math.Inf(1) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config[*testObj]{
				Client:   fake.NewClientBuilder().Build(),
				Services: []Service[*testObj]{&testService{name: "step"}},
			}
			cfg.ApplyDefaults()
			tt.change(&cfg.Backoff)
			require.Error(t, cfg.Validate())
			_, err := NewForConfig(cfg)
			require.Error(t, err)
		})
	}
}

func TestReconcilerBackoffLifecycle(t *testing.T) {
	ctx := context.Background()
	item := &testObj{ObjectMeta: metav1.ObjectMeta{
		Name: "demo", Namespace: "default", Generation: 1, Finalizers: []string{"finalizer"},
	}}
	kube := fake.NewClientBuilder().WithScheme(newTestScheme()).WithStatusSubresource(&testObj{}).WithObjects(item).Build()
	calls := []string{}
	svc := &testService{name: "step", applyErr: errors.New("waiting"), calls: &calls}
	r, err := NewForConfig(Config[*testObj]{
		Client: kube, Services: []Service[*testObj]{svc},
		Backoff: BackoffConfig{
			MinDelay: metav1.Duration{Duration: time.Second},
			MaxDelay: metav1.Duration{Duration: 3 * time.Second},
			Factor:   2,
		},
	})
	require.NoError(t, err)
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(item)}
	reconcile := func(delay time.Duration) {
		t.Helper()
		result, err := r.Reconcile(ctx, req)
		require.NoError(t, err)
		require.Equal(t, delay, result.RequeueAfter)
	}

	reconcile(time.Second)
	reconcile(2 * time.Second)
	reconcile(3 * time.Second)
	require.Len(t, calls, 3) // Normal watch-triggered reconciles are not delayed internally.

	require.NoError(t, kube.Get(ctx, req.NamespacedName, item))
	item.Generation++
	require.NoError(t, kube.Update(ctx, item))
	reconcile(time.Second)
	require.Equal(t, 1, backoffSize(&r.backoff))

	svc.applyErr = nil
	reconcile(0)
	require.Zero(t, backoffSize(&r.backoff))
	svc.applyErr = errors.New("waiting")
	reconcile(time.Second)
	reconcile(2 * time.Second)

	require.NoError(t, kube.Get(ctx, req.NamespacedName, item))
	require.NoError(t, kube.Delete(ctx, item))
	svc.deleteErr = errors.New("waiting")
	reconcile(time.Second)
	reconcile(2 * time.Second)
	svc.deleteErr = nil
	reconcile(0)
	require.Zero(t, backoffSize(&r.backoff))

	// NotFound also removes state, even if deletion happened outside the workflow.
	r.backoff.next(item)
	reconcile(0)
	require.Zero(t, backoffSize(&r.backoff))
}

func TestReconcilerGetErrorClearsBackoff(t *testing.T) {
	ctx := context.Background()
	getErr := errors.New("API unavailable")
	kube := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return getErr
		},
	}).Build()
	r, err := NewForConfig(Config[*testObj]{
		Client: kube, Services: []Service[*testObj]{&testService{name: "step"}},
	})
	require.NoError(t, err)
	item := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo"}}
	r.backoff.next(item)
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "demo"}})
	require.ErrorIs(t, err, getErr)
	require.Zero(t, backoffSize(&r.backoff))
}

func TestReconcilerFinalizerExitsClearBackoff(t *testing.T) {
	now := metav1.Now()
	updateErr := errors.New("update failed")
	for _, tt := range []struct {
		name       string
		finalizers []string
		deleting   bool
		updateErr  error
	}{
		{name: "add finalizer"},
		{name: "failed finalizer update", updateErr: updateErr},
		{name: "deletion without our finalizer", finalizers: []string{"other"}, deleting: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			item := &testObj{ObjectMeta: metav1.ObjectMeta{Name: "demo", Finalizers: tt.finalizers}}
			if tt.deleting {
				item.DeletionTimestamp = &now
			}
			kube := fake.NewClientBuilder().WithScheme(newTestScheme()).WithObjects(item).WithInterceptorFuncs(interceptor.Funcs{
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if tt.updateErr != nil {
						return tt.updateErr
					}
					return c.Update(ctx, obj, opts...)
				},
			}).Build()
			r, err := NewForConfig(Config[*testObj]{
				Client: kube, Services: []Service[*testObj]{&testService{name: "step"}},
			})
			require.NoError(t, err)
			r.backoff.next(item)
			result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(item)})
			if tt.updateErr != nil {
				require.ErrorIs(t, err, tt.updateErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, ctrl.Result{}, result)
			require.Zero(t, backoffSize(&r.backoff))
		})
	}
}

func TestBackoffConfigYAML(t *testing.T) {
	raw := []byte("backoff:\n  minDelay: 1s\n  maxDelay: 1m\n  factor: 1.5\n")
	var cfg Config[*testObj]
	require.NoError(t, yaml.UnmarshalStrict(raw, &cfg))
	cfg.Backoff.ApplyDefaults()
	require.NoError(t, cfg.Backoff.Validate())
	require.Equal(t, time.Second, cfg.Backoff.MinDelay.Duration)
	require.Equal(t, time.Minute, cfg.Backoff.MaxDelay.Duration)
	require.Equal(t, 1.5, cfg.Backoff.Factor)
	encoded, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	var decoded Config[*testObj]
	require.NoError(t, yaml.UnmarshalStrict(encoded, &decoded))
	require.Equal(t, cfg.Backoff, decoded.Backoff)
	require.Error(t, yaml.UnmarshalStrict([]byte("backoff:\n  minDelay: invalid\n"), &decoded))
}
