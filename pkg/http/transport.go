package http

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/ing-bank/golibs/pkg/tlsclient"
)

type TransportOption = func(transport *http.Transport) error

func NewTransport(options ...TransportOption) (*http.Transport, error) {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return transport, ApplyTransportOptions(transport, options)
}

func ApplyTransportOptions(transport *http.Transport, options []TransportOption) error {
	for _, option := range options {
		if err := option(transport); err != nil {
			return err
		}
	}
	return nil
}

// WithProxy sets a proxy URL for the HTTP transport.
func WithProxy(proxy string) TransportOption {
	return func(t *http.Transport) error {
		u, err := url.Parse(proxy)
		if err != nil {
			return err
		}

		t.Proxy = http.ProxyURL(u)
		return nil
	}
}

// WithTLSConfig sets a custom TLS configuration for the HTTP transport.
func WithTLSConfig(tlsconfig *tls.Config) TransportOption {
	return func(c *http.Transport) error {
		c.TLSClientConfig = tlsconfig
		return nil
	}
}

// WithTLS sets up mutual TLS for the HTTP transport using the provided certificate, key, and optional CA certificates.
func WithTLS(cert, key string, cacerts ...string) TransportOption {
	return func(c *http.Transport) error {
		cfg, err := tlsclient.New(cert, key, false, cacerts...)
		if err != nil {
			return err
		}

		c.TLSClientConfig = cfg
		return nil
	}
}

// InsecureSkipVerify configures the HTTP transport to skip TLS certificate verification.
func InsecureSkipVerify() TransportOption {
	return func(t *http.Transport) error {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		} else {
			t.TLSClientConfig.InsecureSkipVerify = true
		}
		return nil
	}
}
