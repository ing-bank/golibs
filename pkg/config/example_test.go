package config

import (
	"errors"
	"fmt"
	"reflect"
)

type ExampleConfig struct {
	Example struct {
		Key string `json:"key"`
	} `json:"example"`
}

func (c ExampleConfig) Validate() error {
	if c.Example.Key == "" {
		return errors.New("key is required") //nolint:err113
	}
	return nil
}

func ExampleLoad() {
	// Set up a fake YAML configuration in /tmp
	// The YAML will be converted to JSON when unmarshalling the configuration
	f, err := generateConfig()
	if err != nil {
		panic(err)
	}

	// We load configuration from file called f.Name() and generate *ExampleConfig type
	// ExampleConfig.Validate() is called automatically as part of loading the configuration
	cfg, err := LoadType[ExampleConfig](f.Name()) // Path to config is optional
	if err != nil {
		panic(err)
	}

	fmt.Println(f.Name())
	fmt.Println(cfg.Example.Key)
	fmt.Println(reflect.TypeFor[*ExampleConfig]())
	// Output:
	// /tmp/golibs-unittest-config.yaml
	// value
	// *config.ExampleConfig
}

type Bar struct {
	Name   string
	Count  int
	Active bool
}

func WithName(name string) Opt[*Bar] {
	return func(b *Bar) error {
		b.Name = name
		return nil
	}
}

func WithCount(count int) Opt[*Bar] {
	return func(b *Bar) error {
		b.Count = count
		return nil
	}
}

func WithActive(active bool) Opt[*Bar] {
	return func(b *Bar) error {
		b.Active = active
		return nil
	}
}

func ExampleOpt() {
	bar := &Bar{}
	_ = ApplyOpts(bar,
		WithName("example-bar"),
		WithCount(42),
		WithActive(true))
	fmt.Printf("%s %d %t\n", bar.Name, bar.Count, bar.Active)
	// Output: example-bar 42 true
}
