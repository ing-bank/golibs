package tlsutils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExampleGenerateTestCertificate() {
	// Generate a test certificate with a CommonName and DNS SANs
	certPath, keyPath, cleanup, err := GenerateTestCertificate("example.com", []string{"www.example.com", "api.example.com"})
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// Use the certificate paths to create a TLS config
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Certificate loaded successfully with %d certificate(s)\n", len(cert.Certificate))
	// Output: Certificate loaded successfully with 1 certificate(s)
}


func TestGenerateTestCertificate(t *testing.T) {
	t.Run("generates certificate with CN only", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("test.example.com", nil)
		require.NoError(t, err)
		defer cleanup()

		// Verify files exist
		assert.FileExists(t, certPath)
		assert.FileExists(t, keyPath)

		// Verify certificate can be loaded
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)
		assert.NotNil(t, cert)

		// Parse and verify CN is added to DNS SANs
		x509Cert, err := ParseCertificateFromPair(cert)
		require.NoError(t, err)
		assert.Equal(t, "test.example.com", x509Cert.Subject.CommonName)
		assert.Contains(t, x509Cert.DNSNames, "test.example.com", "CN should be automatically added to DNS SANs")
	})

	t.Run("generates certificate with CN and DNS SANs", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("test.example.com", []string{"alt1.example.com", "alt2.example.com"})
		require.NoError(t, err)
		defer cleanup()

		// Verify files exist
		assert.FileExists(t, certPath)
		assert.FileExists(t, keyPath)

		// Verify certificate can be loaded and parsed
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)
		assert.NotNil(t, cert)

		// Parse and verify the certificate details
		x509Cert, err := ParseCertificateFromPair(cert)
		require.NoError(t, err)

		assert.Equal(t, "test.example.com", x509Cert.Subject.CommonName)
		assert.Contains(t, x509Cert.DNSNames, "test.example.com", "CN should be automatically added to DNS SANs")
		assert.Contains(t, x509Cert.DNSNames, "alt1.example.com")
		assert.Contains(t, x509Cert.DNSNames, "alt2.example.com")
		assert.Len(t, x509Cert.DNSNames, 3)
	})

	t.Run("generates certificate with IP address as CN", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("127.0.0.1", nil)
		require.NoError(t, err)
		defer cleanup()

		// Verify certificate can be loaded and parsed
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)

		x509Cert, err := ParseCertificateFromPair(cert)
		require.NoError(t, err)

		assert.Equal(t, "127.0.0.1", x509Cert.Subject.CommonName)
		assert.Len(t, x509Cert.IPAddresses, 1, "IP CN should be added to IP SANs")
		assert.Equal(t, "127.0.0.1", x509Cert.IPAddresses[0].String())
		assert.Len(t, x509Cert.DNSNames, 0, "IP CN should not be added to DNS SANs")
	})

	t.Run("generates certificate with IPv6 address as CN", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("::1", nil)
		require.NoError(t, err)
		defer cleanup()

		// Verify certificate can be loaded and parsed
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)

		x509Cert, err := ParseCertificateFromPair(cert)
		require.NoError(t, err)

		assert.Equal(t, "::1", x509Cert.Subject.CommonName)
		assert.Len(t, x509Cert.IPAddresses, 1, "IPv6 CN should be added to IP SANs")
		assert.Equal(t, "::1", x509Cert.IPAddresses[0].String())
	})

	t.Run("does not duplicate CN in DNS SANs", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("test.example.com", []string{"test.example.com", "alt.example.com"})
		require.NoError(t, err)
		defer cleanup()

		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)

		x509Cert, err := ParseCertificateFromPair(cert)
		require.NoError(t, err)

		// Count occurrences of CN in DNS SANs
		count := 0
		for _, dns := range x509Cert.DNSNames {
			if dns == "test.example.com" {
				count++
			}
		}
		assert.Equal(t, 1, count, "CN should appear only once in DNS SANs")
		assert.Len(t, x509Cert.DNSNames, 2)
	})

	t.Run("cleanup removes temporary files", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("test.example.com", nil)
		require.NoError(t, err)

		// Verify files exist before cleanup
		assert.FileExists(t, certPath)
		assert.FileExists(t, keyPath)

		// Call cleanup
		cleanup()

		// Verify files are removed
		_, err = os.Stat(certPath)
		assert.True(t, os.IsNotExist(err), "certificate file should be removed after cleanup")

		_, err = os.Stat(keyPath)
		assert.True(t, os.IsNotExist(err), "key file should be removed after cleanup")
	})

	t.Run("can extract CN and DNS names from generated certificate", func(t *testing.T) {
		certPath, keyPath, cleanup, err := GenerateTestCertificate("primary.example.com", []string{"alt1.example.com", "alt2.example.com"})
		require.NoError(t, err)
		defer cleanup()

		// Load and parse certificate
		keypair, err := tls.LoadX509KeyPair(certPath, keyPath)
		require.NoError(t, err)

		x509Cert, err := ParseCertificateFromPair(keypair)
		require.NoError(t, err)

		// Extract CNs and DNS names
		cns := ExtractCommonNamesAndDNSNames([]*x509.Certificate{x509Cert})

		// Verify extraction
		assert.Contains(t, cns, "primary.example.com")
		assert.Contains(t, cns, "alt1.example.com")
		assert.Contains(t, cns, "alt2.example.com")
		assert.Len(t, cns, 3)
	})
}

