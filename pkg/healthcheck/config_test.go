package healthcheck

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ing-bank/golibs/pkg/store/utilities/timed"
	"github.com/ing-bank/golibs/pkg/task/job"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if len(cfg.Jobs) != 0 {
		t.Error("default Jobs should be empty")
	}
	if cfg.CacheConfig.SyncPeriod != (metav1.Duration{Duration: DefaultSyncPeriod}) {
		t.Errorf("expected SyncPeriod %v, got %v", DefaultSyncPeriod, cfg.CacheConfig.SyncPeriod)
	}
	if cfg.CacheConfig.MaxAge.Duration < cfg.CacheConfig.SyncPeriod.Duration {
		t.Errorf("expected MaxAge greater than SyncPeriod, got MaxAge %v and SyncPeriod %v", cfg.CacheConfig.MaxAge, cfg.CacheConfig.SyncPeriod)
	}
}

func TestJobConfigEmptyEndpoints(t *testing.T) {
	jobCfg := JobConfig{}
	if len(jobCfg.Endpoints) != 0 {
		t.Error("Endpoints should be empty by default")
	}
}

func TestConfigCacheConfigJSONRoundTrip(t *testing.T) {
	cfg := &Config{}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var out Config
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	// CacheConfig is a value, not a pointer, so it should not be nil
}

func TestJobConfigCustomJobConfigFields(t *testing.T) {
	jobCfg := &JobConfig{
		Config: job.Config{
			Timeout: 7 * time.Second,
			MayFail: true,
		},
	}
	if jobCfg.Timeout != 7*time.Second {
		t.Errorf("Timeout should be 7s, got %v", jobCfg.Timeout)
	}
	if !jobCfg.MayFail {
		t.Error("MayFail should be true")
	}
}

func TestConfigCacheConfigZeroValues(t *testing.T) {
	cfg := &Config{
		CacheConfig: timed.Config{},
	}
	if cfg.CacheConfig.SyncPeriod.Duration != 0 {
		t.Errorf("SyncPeriod should be zero, got %v", cfg.CacheConfig.SyncPeriod)
	}
	if cfg.CacheConfig.MaxAge.Duration != 0 {
		t.Errorf("MaxAge should be zero, got %v", cfg.CacheConfig.MaxAge)
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		cfg     Config
		wantErr bool
	}

	cases := []testCase{
		{
			name: "valid config",
			cfg: Config{
				Interval:    metav1.Duration{Duration: 5 * time.Second},
				CacheConfig: timed.Config{SyncPeriod: metav1.Duration{Duration: 10 * time.Second}, MaxAge: metav1.Duration{Duration: 20 * time.Second}},
			},
			wantErr: false,
		},
		{
			name:    "interval zero",
			cfg:     Config{Interval: metav1.Duration{}, CacheConfig: timed.Config{SyncPeriod: metav1.Duration{Duration: 10 * time.Second}, MaxAge: metav1.Duration{Duration: 20 * time.Second}}},
			wantErr: true,
		},
		{
			name:    "interval negative",
			cfg:     Config{Interval: metav1.Duration{Duration: -1 * time.Second}, CacheConfig: timed.Config{SyncPeriod: metav1.Duration{Duration: 10 * time.Second}, MaxAge: metav1.Duration{Duration: 20 * time.Second}}},
			wantErr: true,
		},
		{
			name:    "interval greater than sync period",
			cfg:     Config{Interval: metav1.Duration{Duration: 15 * time.Second}, CacheConfig: timed.Config{SyncPeriod: metav1.Duration{Duration: 10 * time.Second}, MaxAge: metav1.Duration{Duration: 20 * time.Second}}},
			wantErr: true,
		},
		{
			name:    "invalid cache config",
			cfg:     Config{Interval: metav1.Duration{Duration: 5 * time.Second}, CacheConfig: timed.Config{SyncPeriod: metav1.Duration{}, MaxAge: metav1.Duration{}}}, // assuming timed.Config.Validate fails on zero
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
