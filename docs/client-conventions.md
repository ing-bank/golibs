# Constructor, Config and Option Conventions

This document describes the recommended patterns for new packages in `golibs`.

The goals are:

- Consistent package APIs
- YAML-friendly configuration
- Minimal boilerplate
- Backward compatibility with existing libraries
- Clear separation between configuration and runtime customization

# Quick Start

```go
type Config struct {
    Message string `json:"message" yaml:"message"`
}

func (c *Config) ApplyDefaults() {
    if c.Message == "" {
        c.Message ="hello"
    }
}

func (c *Config) Validate() error {
    if c.Message == "" {
        return errors.New("message is required")
    }
    return nil
}

type Client struct {
    Message string
}

type ClientOption = config.Option[*Client]

func New(opts ...ClientOption) (*Client, error) {
    return NewForConfig(Config{}, opts...)
}

func NewForConfig(cfg Config, opts ...ClientOption) (*Client, error) {
    if err := config.Configure(&cfg); err != nil {
        return nil, err
    }

    client := &Client{
        Message: cfg.Message,
    }

    return client, config.ApplyOptions(client, opts...)
}
```

Lifecycle:

```text
Config
  ↓
ApplyDefaults()
  ↓
Prepare()
  ↓
Validate()
  ↓
Construct Client
  ↓
Apply ClientOptions
  ↓
Usable Client
```

# Overview

A package should generally contain:

```go
type Config struct{}

type Client struct{}

type ClientOption func(*Client) error
```

Configuration is used to **build** a client.

Client options are used to **customize** a constructed client.

The lifecycle is:

```text
Config
  ↓
ApplyDefaults()
  ↓
Prepare()
  ↓
Validate()
  ↓
Construct Client
  ↓
Apply ClientOptions
  ↓
Usable Client
```

# Configuration Lifecycle

Configuration may optionally implement the following methods:

```go
func (c *Config) ApplyDefaults()
func (c *Config) Prepare() error
func (c *Config) Validate() error
```

## ApplyDefaults
```go
func (c *Config) ApplyDefaults()
```

Responsibilities:

- Populate default values
- Never perform I/O
- Never return an error.

Example

```go
func (c *Config) ApplyDefaults() {
    if c.Timeout == 0 {
        c.Timeout = 30 * time.Second
    }
}
```

## Prepare

```go
func (c *Config) Prepare() error {
    return nil
}
```

Responsibilities:

- Load local resources.
- Derive values from configuration.
- Perform local filesystem I/O if necessary.

Examples:

- Loading certificates from disk.
- Reading local files.
- Deriving Common Names from certificates.

Avoid:

- Network requests.
- External service calls.
- Anything requiring cancellation or tracing.

Since `Prepare()` does not receive a `context.Context`, it should remain focused on local preparation work.

# Validate

```go
func (c *Config) Validate() error
```

Responsibilities:

- Verify the final prepared configuration.
- Return clear validation errors.

Validation runs after defaults and preparation.

## Configure Helper

The shared config package provides:

```go
config.Configure(&cfg)
```

which executes:

```text
ApplyDefaults()
Prepare() error
Validate()
```

Interfaces are optional, simple configs may implement none of them.

## Idempotency

These methods should be idempotent whenever possible:

```go
ApplyDefaults()
Prepare()
Validate()
```

Calling them multiple times should not cause unexpected behavior.

Good:

```go
cfg.Prepare()
cfg.Prepare()
```

Bad:

```go
cfg.Prepare() // adds CA
cfg.Prepare() // adds same CA again
```

# Constructors

Every package should expose:

```go
func NewClient(opts ...ClientOption) (*Client, error)
```

and

```go
func NewClientForConfig(cfg Config, opts ...ClientOption) (*Client, error)
```

Recommended implementation:

```go
func NewClient(opts ...ClientOption) (*Client, error) {
    return NewClientForConfig(Config{}, opts...)
}
```

```go
func NewClientForConfig(cfg Config, opts ...ClientOption) (*Client, error) {
    if err := config.Configure(&cfg); err != nil {
        return nil, err
    }
    
    client := &Client{
        // construct from cfg
    }
    
    if err := config.ApplyOptions(client, opts...); err != nil {
        return nil, err
    }
    
    return client, nil
}
```

Constructors should always call `config.Configure()` to ensure configuration has been defaulted, prepared and validated before use.

# Value Configs vs Pointer Configs

Prefer:

```go
func NewClientForConfig(cfg Config, opts ...ClientOption)
```

instead of:

```go
func NewClientForConfig(cfg *Config, opts ...ClientOption)
```

Reasons:

- Makes copying explicit.
- Avoids unexpected mutation of caller-owned configs.
- Clearly communicates constructor ownership.

The constructor is free to modify its private copy during construction without affecting the caller.

# Configuration vs Runtime State

As a rule:

## Config

Represents user input.

Examples:

```go
type Config struct {
    TLS        *tlsclient.Config
    FollowRedirect bool
    DefaultHeaders map[string]string
}
```

## Client

Represents runtime state.
Examples:

```go
type Client struct {
    Http                  *http.Client
    Tripperware        *tripperware.Tripperware
    DefaultRequestOptions []RequestOption
}
```

Construction translates configuration into runtime state.

# Exceptions

Some packages do not naturally have a client type. For example `tripperware` returns a function chain.

In these cases:

- Configuration conventions still apply.
- Options may operate on config, or options may not be provided at all.
  Use judgement where a package does not fit the typical client/config pattern.

# Summary

Preferred package shape:

```go
type Config struct{}

type Client struct{}

type ClientOption func(*Client) error
func (c *Config) ApplyDefaults()
func (c *Config) Prepare() error
func (c *Config) Validate() error

func NewClient(opts ...ClientOption) (*Client, error)

func NewClientForConfig(
    cfg Config,
    opts ...ClientOption,
) (*Client, error)
`*`

Core rule:

> Configuration builds the client. Options customize the client.
```