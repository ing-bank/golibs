// Package tlsserver provides utilities for creating and configuring TLS server configurations.
//
// It simplifies the creation of TLS configurations for server connections with support for
// mutual TLS (mTLS), client certificate validation, and flexible TLS version configuration.
//
// Core Features:
//
//   - Server Certificate Management: Load server certificates and keys for TLS.
//   - Mutual TLS Support: Configure client authentication and validation via client certificates.
//   - Client Certificate Validation: Support for optional, required, or verified client auth modes.
//   - Custom CA Certificates: Configure CA certificates for client certificate verification.
//   - TLS Version Control: Specify minimum TLS version to enforce security policies.
//   - Certificate Pool Management: Automatically creates and manages X.509 certificate pools.
//
// Basic Usage:
//
//	import "github.com/ing-bank/golibs/pkg/tlsserver"
//
//	// Create a TLS config from a Config struct (recommended)
//	cfg := &tlsserver.Config{
//		Cert:             "server.crt",
//		Key:              "server.key",
//		RootCAs:          []string{"ca.crt"},
//		MinVersion:       "1.2",
//		ClientAuthType:   "VerifyClientCertIfGiven",
//	}
//	tlsConfig, err := tlsserver.NewForConfig(cfg)
//	if err != nil {
//		// handle error
//	}
//
//	// Use with HTTP server
//	server := &http.Server{
//		Addr:      ":443",
//		TLSConfig: tlsConfig,
//		Handler:   mux,
//	}
//	server.ListenAndServeTLS("", "")
//
// Quick Configuration:
//
// For simple cases without custom options, use New:
//
//	tlsConfig, err := tlsserver.New(
//		"server.crt",
//		"server.key",
//		"ca.crt",
//	)
//
// Client Authentication Modes:
//
// The Config struct supports different client authentication modes via ClientAuthType:
//
//   - NoClientCert: Client certificates are neither requested nor verified.
//   - RequestClientCert: Client certificates are requested but not required.
//   - RequireClientCert: Client certificates are required but not verified.
//   - VerifyClientCertIfGiven: Client certificates are optional but verified if provided.
//   - RequireAndVerifyClientCert: Client certificates are required and verified.
//
// Related Packages:
//
// - pkg/tlsclient: Client-side TLS configuration
// - pkg/tlsutils: Low-level X.509 certificate utilities
package tlsserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/tlsutils"
)

func New(cert, key string, cacerts ...string) (*tls.Config, error) {
	cfg := &Config{
		Config: *tlsutils.NewConfig(cert, key, cacerts...),
	}
	tlsConfig, err := NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

func NewForConfig(cfg *Config, opts ...config.Option[*tls.Config]) (*tls.Config, error) {
	// apply default values to the configuration
	cfg.ApplyDefaults()

	clientAuthType, err := cfg.ParseClientAuthType()
	if err != nil {
		return nil, err
	}

	var pool *x509.CertPool
	var keypair tls.Certificate

	if cfg.UseTLS() {
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
			MinVersion:   minVersion, //nolint:gosec // MinVersion is validated by tlsutils.ParseTLSVersion
			Certificates: []tls.Certificate{keypair},
			ClientCAs:    pool,
			ClientAuth:   clientAuthType,
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
