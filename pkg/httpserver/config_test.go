package httpserver

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApplyDefaults_NilConfig(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ApplyDefaults(nil) did not panic as expected")
		}
	}()
	var cfg *Config = nil
	cfg.ApplyDefaults()
}

func TestApplyDefaults_ZeroFields(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	cfg.ApplyDefaults()
	want := DefaultConfig()
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("ApplyDefaults(zero fields) = %+v, want %+v", cfg, want)
	}
}

func TestApplyDefaults_PartialFields(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Host:           "localhost",
		Port:           1234,
		ReadTimeout:    metav1.Duration{},
		WriteTimeout:   metav1.Duration{Duration: 5 * time.Second},
		MaxHeaderBytes: 0,
	}
	cfg.ApplyDefaults()
	if cfg.Host != "localhost" {
		t.Errorf("Host should not be overwritten, got %v", cfg.Host)
	}
	if cfg.Port != 1234 {
		t.Errorf("Port should not be overwritten, got %v", cfg.Port)
	}
	if cfg.ReadTimeout.Duration != DefaultReadTimeout {
		t.Errorf("ReadTimeout should be default, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout.Duration != 5 * time.Second {
		t.Errorf("WriteTimeout should not be overwritten, got %v", cfg.WriteTimeout)
	}
	if cfg.MaxHeaderBytes != DefaultMaxHeaderBytes {
		t.Errorf("MaxHeaderBytes should be default, got %v", cfg.MaxHeaderBytes)
	}
}

func TestApplyDefaults_AllFieldsSet(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Host:              "customhost",
		Port:              9999,
		ReadTimeout:       metav1.Duration{Duration: 1 * time.Second},
		ReadHeaderTimeout: metav1.Duration{Duration: 2 * time.Second},
		WriteTimeout:      metav1.Duration{Duration: 3 * time.Second},
		ShutdownTimeout:   metav1.Duration{Duration: 4 * time.Second},
		MaxHeaderBytes:    2048,
	}
	cfg.ApplyDefaults()
	want := &Config{
		Host:              "customhost",
		Port:              9999,
		ReadTimeout:       metav1.Duration{Duration: 1 * time.Second},
		ReadHeaderTimeout: metav1.Duration{Duration: 2 * time.Second},
		WriteTimeout:      metav1.Duration{Duration: 3 * time.Second},
		ShutdownTimeout:   metav1.Duration{Duration: 4 * time.Second},
		MaxHeaderBytes:    2048,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("ApplyDefaults(all fields set) should not change values, got %+v, want %+v", cfg, want)
	}
}
