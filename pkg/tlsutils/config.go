package tlsutils

import (
	"crypto/tls"
	"fmt"
)

const (
	DefaultMinVersion = tls.VersionTLS12
)

const (
	// TLSNoClientCert indicates that no client certificate should be requested
	// during the handshake, and if any certificates are sent they will not
	// be verified.
	TLSNoClientCert = "NoClientCert"
	// TLSVerifyClientCertIfGiven indicates that a client certificate should be requested
	// during the handshake, but does not require that the client sends a
	// certificate. If the client does send a certificate it is required to be
	// valid.
	TLSVerifyClientCertIfGiven = "VerifyClientCertIfGiven"
	// TLSRequestClientCert indicates that a client certificate should be requested
	// during the handshake, but does not require that the client send any
	// certificates.
	TLSRequestClientCert = "RequestClientCert"
	// TLSRequireAndVerifyClientCert indicates that a client certificate should be requested
	// during the handshake, and that at least one valid certificate is required
	// to be sent by the client.
	TLSRequireAndVerifyClientCert = "RequireAndVerifyClientCert"
	// TLSRequireAnyClientCert indicates that a client certificate should be requested
	// during the handshake, and that at least one certificate is required to be
	// sent by the client, but that certificate is not required to be valid.
	TLSRequireAnyClientCert = "RequireAnyClientCert"

	TLSVersion10 = "TLS 1.0"
	TLSVersion11 = "TLS 1.1"
	TLSVersion12 = "TLS 1.2"
	TLSVersion13 = "TLS 1.3"
)

type Config struct {
	Cert       string   `yaml:"cert" json:"cert"`
	Key        string   `yaml:"key" json:"key"`
	RootCAs    []string `yaml:"rootCAs" json:"rootCAs"`
	MinVersion string   `yaml:"minVersion"`
	Disabled   bool     `yaml:"disabled" json:"disabled"`
}

func DefaultConfig() *Config {
	return &Config{
		MinVersion: tls.VersionName(DefaultMinVersion),
	}
}

func (c *Config) ApplyDefaults() {
	if c.MinVersion == "" {
		c.MinVersion = tls.VersionName(DefaultMinVersion)
	}
}

func ParseClientAuthType(clientAuthType string) (tls.ClientAuthType, error) {
	switch clientAuthType {
	case TLSRequireAndVerifyClientCert:
		return tls.RequireAndVerifyClientCert, nil
	case TLSVerifyClientCertIfGiven:
		return tls.VerifyClientCertIfGiven, nil
	case TLSRequestClientCert:
		return tls.RequestClientCert, nil
	case TLSNoClientCert:
		return tls.NoClientCert, nil
	case TLSRequireAnyClientCert:
		return tls.RequireAnyClientCert, nil
	default:
		return 0, fmt.Errorf("unknown client auth type: %s", clientAuthType)
	}
}

func ParseTLSVersion(version string) (uint16, error) {
	switch version {
	case TLSVersion10:
		return tls.VersionTLS10, nil
	case TLSVersion11:
		return tls.VersionTLS11, nil
	case TLSVersion12:
		return tls.VersionTLS12, nil
	case TLSVersion13:
		return tls.VersionTLS13, nil
	default:
		return 0, nil
	}
}

func NewConfig(cert, key string, cacerts ...string) *Config {
	return &Config{
		Cert:       cert,
		Key:        key,
		RootCAs:    cacerts,
		MinVersion: tls.VersionName(DefaultMinVersion),
	}
}

func (c *Config) UseTLS() bool {
	if c == nil {
		return false
	}
	if c.Cert != "" && c.Key != "" {
		return !c.Disabled
	}
	return false
}
