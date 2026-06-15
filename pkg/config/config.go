// Package config provides utilities for loading and validating configuration from files.
//
// It supports loading configuration from YAML or JSON files, with automatic validation
// and default value application. The package defines interfaces that configuration types
// should implement:
//
//   - Validatable: Types implementing Validate() error can be validated after loading.
//   - Defaulter: Types implementing ApplyDefaults() can set default values.
//   - CommandLineInterface: Types implementing BindFlags(fs *pflag.FlagSet) error can bind
//     command-line flags for configuration options.
//
// Basic usage:
//
//		type MyConfig struct {
//			Name string `yaml:"name"`
//			Port int    `yaml:"port"`
//		}
//
//		func (c *MyConfig) Validate() error {
//			if c.Name == "" {
//				return errors.New("name is required")
//			}
//			return nil
//		}
//
//	 // Optional, will be called if set
//		func (c *MyConfig) ApplyDefaults() {
//			if c.Port == 0 {
//				c.Port = 8080
//			}
//		}
//
//		// Load and validate in one call
//		cfg, err := LoadType[MyConfig]("/path/to/config.yaml")
//		if err != nil {
//			// handle error
//		}
//
// Configuration files are loaded from disk (default: /config/config.yaml) and can be
// either YAML or JSON format. YAML files are automatically converted to JSON before
// unmarshalling. Multiple configurations can be validated together using ChainValidations.
package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var ErrValidation = errors.New("validation error")

type Defaulter interface {
	ApplyDefaults()
}

type Validatable interface {
	Validate() error
}

type CommandLineInterface interface {
	Validate() error
	BindFlags(fs *pflag.FlagSet) error
}

// ChainValidations is a convenience function to execute multiple validations as a one-liner
func ChainValidations(configurations ...Validatable) error {
	for _, configuration := range configurations {
		if err := configuration.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func ChainFlags(fs *pflag.FlagSet, configurations ...CommandLineInterface) error {
	for _, configuration := range configurations {
		if err := configuration.BindFlags(fs); err != nil {
			return err
		}
	}
	return nil
}

type ConfigurableConstraint[T any] interface {
	Validatable
	*T // Specify explicitly that T can be anything, but a *T would satisfy Validatable. Needed to call `Load`
}

// LoadType works the same as Load, but also creates a new configuration object to make loading
// configuration a one-liner
func LoadType[T any, PT ConfigurableConstraint[T]](path ...string) (*T, error) {
	target := PT(new(T))
	return target, Load(target, path...)
}

// Load calls LoadOnly to load a configuration from a file (YAML or JSON), applies default if the config supports it,
// validates it, and returns an error if loading or validation fails.
func Load(target Validatable, path ...string) error {
	if err := LoadOnly(target, path...); err != nil {
		return err
	}
	if defaulter, ok := target.(Defaulter); ok {
		defaulter.ApplyDefaults()
	}
	return target.Validate()
}

// LoadOnly loads a configuration stored on filesystem under path (or default /config/config.yaml). The configuration
// can either be YAML or JSON. In case of YAML it would be converted to JSON and then unmarshalled to desired type.
func LoadOnly(target Validatable, path ...string) error {
	pathToConfig := opt.Opt("/config/config.yaml", path)

	// Clean the path to remove any ../ or other unsafe elements
	cleanPath := filepath.Clean(pathToConfig)

	raw, err := os.ReadFile(cleanPath)
	if err != nil {
		return err
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(raw), 4096)
	return decoder.Decode(target)
}
