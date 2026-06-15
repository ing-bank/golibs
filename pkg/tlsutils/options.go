package tlsutils

import (
	"crypto/tls"
	"crypto/x509"
	"slices"

	"github.com/ing-bank/golibs/pkg/config"
)

type VerifyPeerCertificateFunc func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error

func WithCertificates(cert ...tls.Certificate) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.Certificates = slices.Concat(t.Certificates, cert)
		return nil
	}
}

func WithRootCAPools(pool *x509.CertPool) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.RootCAs = pool
		return nil
	}
}

func WithMinVersion(version uint16) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.MinVersion = version
		return nil
	}
}

func WithClientAuth(authType tls.ClientAuthType) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.ClientAuth = authType
		return nil
	}
}

func WithClientCAs(pool *x509.CertPool) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.ClientCAs = pool
		return nil
	}
}

func WithVerifyPeerCertificate(verifyPeerCertificate VerifyPeerCertificateFunc) config.Opt[*tls.Config] {
	return func(t *tls.Config) error {
		t.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			return verifyPeerCertificate(rawCerts, verifiedChains)
		}
		return nil
	}
}
