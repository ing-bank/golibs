package full

import (
	"errors"
	"fmt"
	"os"

	"github.com/ing-bank/golibs/pkg/config"
)

type Printer struct {
	cfg *Config
}

func (c *Printer) Print() {
	fmt.Println(c.cfg.Message)
	if c.cfg.ExtraMessage != "" {
		fmt.Println(c.cfg.ExtraMessage)
	}
}

type Config struct {
	Message          string `json:"message" yaml:"message"`
	ExtraMessageFile string `json:"extraMessageFile" yaml:"extraMessageFile"`
	ExtraMessage     string `json:"-" yaml:"-"` // Example for 'Runtime' variable, noy to be loaded from disk
}

var DefaultConfig = config.DefaultConfig[Config]() // Makes new struct and calls ApplyDefaults, returns func () *Config

type Option = config.Option[*Config]

func WithMessage(msg string) Option {
	return func(cfg *Config) error {
		cfg.Message = msg
		return nil
	}
}

// ExtraMessage Options could also be added. Not included in this example for brevity.

func (c *Config) ApplyDefaults() {
	if c.Message == "" {
		c.Message = "Hello, World!"
	}
}

func (c *Config) Prepare() error {
	// Prepare can be used to perform additional local loading of configuration, for example from files or environment variables.
	// In this example we load an extra message from a file, if provided.
	// Prepare should not be used to load configuration from remote sources, since no context is provided this operation
	// cannot be canceled.
	if c.ExtraMessageFile == "" {
		return nil
	}

	raw, err := os.ReadFile(c.ExtraMessageFile)
	if err != nil {
		return fmt.Errorf("read extra message from %q: %w", c.ExtraMessageFile, err)
	}

	c.ExtraMessage = string(raw)
	return nil
}

func (c *Config) Validate() error {
	if c.Message == "" {
		return errors.New("message is required")
	}
	return nil
}

func NewPrinterFromConfig(cfg Config) (*Printer, error) {
	if err := config.Configure(cfg); err != nil {
		return nil, err
	}
	return &Printer{cfg: &cfg}, nil
}

func NewPrinter(opts ...Option) (*Printer, error) {
	cfg, err := config.New[Config](opts...)
	if err != nil {
		return nil, err
	}
	return NewPrinterFromConfig(*cfg)
}

func Example() {
	var printer *Printer
	var err error

	// Two ways to create a client:
	// 1. Via config
	cfg := Config{Message: "Hello from config!"}
	printer, err = NewPrinterFromConfig(cfg)
	if err != nil {
		panic(err)
	}
	printer.Print()
	// We can also load config from a file, for example:
	printer, err = NewPrinterFromConfig(*config.LoadTypeOrDie[Config]("config.yaml"))
	if err != nil {
		panic(err)
	}
	printer.Print()

	// 2. Using options to create a client directly (still uses a config under the hood)
	// Note that options are optional. Without the option we would get "Hello, World!" from
	// the DefaultConfig.
	printer, err = NewPrinter(WithMessage("Hello from options!"))
	if err != nil {
		panic(err)
	}
	printer.Print()

	// Output:
	// Hello from config!
	// Hello from config file!
	// Hello from options!
}
