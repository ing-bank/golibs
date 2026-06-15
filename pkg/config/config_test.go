package config

import (
	"errors"
	"os"
	"testing"
)

type ConfigWithDefaults struct {
	Port    int `json:"port"`
	Timeout int `json:"timeout"`
}

func (c *ConfigWithDefaults) ApplyDefaults() {
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.Timeout == 0 {
		c.Timeout = 30
	}
}

func (c *ConfigWithDefaults) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func TestLoadWithDefaults(t *testing.T) {
	type testCase struct {
		name        string
		fileContent string
		wantPort    int
		wantTimeout int
	}

	tests := []testCase{
		{
			name:        "defaults applied when not in file",
			fileContent: "{}",
			wantPort:    8080,
			wantTimeout: 30,
		},
		{
			name:        "file values override defaults",
			fileContent: "port: 9000\ntimeout: 60\n",
			wantPort:    9000,
			wantTimeout: 60,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a config file with specific values
			f, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			if _, err := f.WriteString(tc.fileContent); err != nil {
				t.Fatal(err)
			}
			f.Close()

			cfg := &ConfigWithDefaults{}
			if err := Load(cfg, f.Name()); err != nil {
				t.Fatalf("Load failed: %v", err)
			}

			if cfg.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", cfg.Port, tc.wantPort)
			}
			if cfg.Timeout != tc.wantTimeout {
				t.Errorf("Timeout = %d, want %d", cfg.Timeout, tc.wantTimeout)
			}
		})
	}
}

func generateConfig() (*os.File, error) {
	f, err := os.Create("/tmp/golibs-unittest-config.yaml")
	if err != nil {
		return f, err
	}

	if _, err = f.WriteString("example:\n  key: value"); err != nil {
		return f, err
	}

	return f, f.Close()
}

func generateConfigOrDie(t *testing.T) *os.File {
	f, err := generateConfig()
	if err != nil {
		t.Fatal(err.Error())
	}
	return f
}

func TestLoad(t *testing.T) {
	f := generateConfigOrDie(t)
	defer os.Remove(f.Name())

	cfg := &ExampleConfig{}
	if err := Load(cfg, f.Name()); err != nil {
		t.Error(err.Error())
	}

	if err := cfg.Validate(); err != nil {
		t.Error(err.Error())
	}
}

func TestLoadType(t *testing.T) {
	f := generateConfigOrDie(t)
	defer os.Remove(f.Name())

	cfg, err := LoadType[ExampleConfig](f.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	if err := cfg.Validate(); err != nil {
		t.Error(err.Error())
	}
}
