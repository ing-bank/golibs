package telnet

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
	"github.com/ing-bank/golibs/pkg/tlsclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ checks.Handler = (*Client)(nil)

const (
	DefaultRequestTimeout = 5 * time.Second
	DefaultProtocol       = "tcp"
)

// Config is the Telnet checker configuration settings container.
type Config struct {
	// Address is the host:port to check (e.g., "localhost:23").
	Address string `yaml:"address" json:"address"`
	// RequestTimeout is the duration to wait for the connection.
	RequestTimeout metav1.Duration `yaml:"timeout" json:"timeout"`
	// TLSConfig is the optional TLS configuration for secure connections.
	TLSConfig tlsclient.Config `yaml:"tls" json:"tls"`
	// Protocol specifies the protocol to use: "tcp" or "udp". Default is "tcp".
	Protocol string `yaml:"protocol" json:"protocol"`
}

func (c *Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("address is required")
	}
	if c.Protocol != "" && c.Protocol != "tcp" && c.Protocol != "udp" {
		return fmt.Errorf("protocol must be either 'tcp' or 'udp'")
	}
	return nil
}

type Client struct {
	dialer    *net.Dialer
	tlsdialer *tls.Dialer
	address   string
	protocol  string
}

// New creates a new Telnet service health check that verifies:
// - connection establishing to the given address
func New(c *Config) (*Client, error) {
	cfg := *c // shallow copy
	if cfg.RequestTimeout.Duration == 0 {
		cfg.RequestTimeout = metav1.Duration{Duration: DefaultRequestTimeout}
	}
	if cfg.Protocol == "" {
		cfg.Protocol = DefaultProtocol
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid telnet health check config: %w", err)
	}

	var tlsdialer *tls.Dialer
	if cfg.TLSConfig.UseTLS() {
		tlsConfig, err := tlsclient.NewForConfig(&c.TLSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config for health check: %w", err)
		}
		tlsdialer = &tls.Dialer{NetDialer: &net.Dialer{Timeout: cfg.RequestTimeout.Duration}, Config: tlsConfig}
	}

	return &Client{
		dialer:    &net.Dialer{Timeout: cfg.RequestTimeout.Duration},
		tlsdialer: tlsdialer,
		address:   cfg.Address,
		protocol:  cfg.Protocol,
	}, nil
}

func NewOrDie(c *Config) *Client {
	h, err := New(c)
	if err != nil {
		panic(err)
	}
	return h
}

func (c *Client) Check(ctx context.Context) (err error) {
	if c.tlsdialer != nil {
		// Establish TLS connection directly
		conn, err := c.tlsdialer.DialContext(ctx, c.protocol, c.address)
		if err != nil {
			return fmt.Errorf("telnet health check TLS dial failed: %w", err)
		}
		return conn.Close()
	}
	// Establish plain TCP connection
	conn, err := c.dialer.DialContext(ctx, c.protocol, c.address)
	if err != nil {
		return fmt.Errorf("telnet health check failed: %w", err)
	}
	return conn.Close()
}
