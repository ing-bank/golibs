// Package tlsclient provides utilities for creating and configuring TLS client configurations.
//
// It simplifies the creation of TLS configurations for client connections with support for
// mutual TLS (mTLS), custom CA certificates, and flexible TLS version configuration.
//
// Core Features:
//
//   - Mutual TLS Support: Load client certificates and keys for mTLS authentication.
//   - Custom CA Certificates: Configure custom root CA certificates for server verification.
//   - TLS Version Control: Specify minimum TLS version to enforce security policies.
//   - Insecure Mode: Optional skip verification for testing or development (use with caution).
//   - Certificate Pool Management: Automatically creates and manages X.509 certificate pools.
//
// Basic Usage:
//
//	import "github.com/ing-bank/golibs/pkg/tlsclient"
//
//	// Create a TLS config from a Config struct (recommended)
//	cfg := &tlsclient.Config{
//		Cert:               "client.crt",
//		Key:                "client.key",
//		RootCAs:            []string{"ca.crt", "intermediate-ca.crt"},
//		MinVersion:         "1.2",
//		InsecureSkipVerify: false,
//	}
//	tlsConfig, err := tlsclient.NewForConfig(cfg)
//	if err != nil {
//		// handle error
//	}
//
//	// Use with HTTP client
//	client := &http.Client{
//		Transport: &http.Transport{
//			TLSClientConfig: tlsConfig,
//		},
//	}
//
// Quick Configuration:
//
// For simple cases without custom options, use New:
//
//	tlsConfig, err := tlsclient.New(
//		"client.crt",
//		"client.key",
//		false, // don't skip verification
//		"ca.crt",
//	)
//
// Related Packages:
//
// - pkg/tlsutils: Low-level X.509 certificate utilities
// - pkg/tlsserver: Server-side TLS configuration
package tlsclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/tlsutils"
)

func New(cert, key string, insecureSkipVerify bool, cacerts ...string) (*tls.Config, error) {
	cfg := NewConfig(cert, key, insecureSkipVerify, cacerts...)
	tlsConfig, err := NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

// NewForConfig creates a new TLS configuration for a client based on the provided Config and options.
func NewForConfig(cfg *Config, opts ...config.Option[*tls.Config]) (*tls.Config, error) {
	//apply default values to the configuration
	cfg.ApplyDefaults()

	var err error
	var pool *x509.CertPool
	var keypair tls.Certificate

	if cfg.Key != "" && cfg.Cert != "" {
		if pool, keypair, err = tlsutils.NewX509KeyPair(cfg.Cert, cfg.Key, cfg.RootCAs...); err != nil {
			return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
		}
	}

	minVersion, err := tlsutils.ParseTLSVersion(cfg.MinVersion)
	if err != nil {
		return nil, err
	}

	tlsConfig := &TLSConfig{
		&tls.Config{
			MinVersion:         minVersion,
			RootCAs:            pool,
			Certificates:       []tls.Certificate{keypair},
			InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // TODO: Validate if API is prod before allowing InsecureSkipVerify
		},
	}

	// apply options to the server
	if err := config.ApplyOpts(tlsConfig.Config, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply TLS option: %w", err)
	}

	// validate the server configuration
	if err := tlsConfig.Validate(); err != nil {
		return nil, fmt.Errorf("failed to create HTTP server: %w", err)
	}

	return tlsConfig.Config, nil
}

type TLSConfig struct {
	*tls.Config
}

func (t *TLSConfig) Validate() error {
	if t == nil || t.Config == nil {
		return fmt.Errorf("tls config is nil")
	}
	return nil
}
