package tlsutils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrCertificateInvalid     = errors.New("certificate invalid")
	ErrCertificateNotParsable = errors.New("failed to parse certificate from server")
)

func NewX509KeyPair(cert, key string, cacerts ...string) (*x509.CertPool, tls.Certificate, error) {
	pool, err := NewCertPool(cacerts)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("failed to create cert pool: %w", err)
	}

	keypair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, tls.Certificate{}, fmt.Errorf("failed to load keypair: %w", err)
	}

	return pool, keypair, err
}

func NewCertPool(cacerts []string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	for _, v := range cacerts {
		// Clean the path to remove any ../ or other unsafe elements
		cleanPath := filepath.Clean(v)

		caCertIngFile, err := os.ReadFile(cleanPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA '%s': %v", v, err)
		}
		if ok := certPool.AppendCertsFromPEM(caCertIngFile); !ok {
			return nil, fmt.Errorf("%w: %s", ErrCertificateInvalid, v)
		}
	}
	return certPool, nil
}

func VerifyPeerCertificate(certPool *x509.CertPool, certificates [][]byte) error {
	certs := make([]*x509.Certificate, len(certificates))
	for i, asn1Data := range certificates {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrCertificateNotParsable, err)
		}
		certs[i] = cert
	}
	opts := x509.VerifyOptions{
		Roots: certPool,
	}
	// ALL certificates are verified only
	for _, cert := range certs {
		if _, err := cert.Verify(opts); err != nil {
			return fmt.Errorf("%w: %s", ErrCertificateInvalid, err)
		}
	}
	return nil
}
