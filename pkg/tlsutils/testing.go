package tlsutils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// GenerateTestCertificate creates a self-signed certificate for testing purposes.
// It generates a certificate with the given CommonName and DNS SANs, writes them to temporary files,
// and returns the paths to the certificate and key files along with a cleanup function.
//
// The CommonName is automatically added to the DNS SANs list. If the CN is an IP address,
// it will be added to the IP SANs instead.
//
// Parameters:
//   - cn: The CommonName for the certificate subject
//   - dnsNames: Optional DNS Subject Alternative Names (SANs)
//
// Returns:
//   - certPath: Path to the generated certificate file
//   - keyPath: Path to the generated private key file
//   - cleanup: Function to clean up the temporary files and directory
//   - err: Any error that occurred during generation
//
// Example:
//
//	certPath, keyPath, cleanup, err := tlsutils.GenerateTestCertificate("test.example.com", []string{"alt.example.com"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cleanup()
func GenerateTestCertificate(cn string, dnsNames []string) (certPath, keyPath string, cleanup func(), err error) {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", nil, err
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", nil, err
	}

	// Prepare DNS names and IP addresses for SANs
	var ipAddresses []net.IP
	allDNSNames := make([]string, 0, len(dnsNames)+1)

	// Add provided DNS names
	allDNSNames = append(allDNSNames, dnsNames...)

	// Add CN to SANs (as DNS name or IP address)
	if ip := net.ParseIP(cn); ip != nil {
		// CN is an IP address
		ipAddresses = append(ipAddresses, ip)
	} else {
		// CN is a DNS name - add it to DNS SANs if not already present
		found := false
		for _, dns := range allDNSNames {
			if dns == cn {
				found = true
				break
			}
		}
		if !found {
			allDNSNames = append(allDNSNames, cn)
		}
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: cn,
		},
		DNSNames:              allDNSNames,
		IPAddresses:           ipAddresses,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", nil, err
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "cert-test-*")
	if err != nil {
		return "", "", nil, err
	}

	// Setup cleanup function that will be called on error or returned to caller
	cleanupDir := func() {
		_ = os.RemoveAll(tmpDir)
	}

	certPath = filepath.Join(tmpDir, "cert.pem")
	keyPath = filepath.Join(tmpDir, "key.pem")

	// Write certificate to file
	if err := writePEMFile(certPath, "CERTIFICATE", certDER); err != nil {
		cleanupDir()
		return "", "", nil, fmt.Errorf("failed to write certificate: %w", err)
	}

	// Marshal and write private key to file
	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		cleanupDir()
		return "", "", nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := writePEMFile(keyPath, "EC PRIVATE KEY", keyBytes); err != nil {
		cleanupDir()
		return "", "", nil, fmt.Errorf("failed to write private key: %w", err)
	}

	return certPath, keyPath, cleanupDir, nil
}

// writePEMFile writes PEM-encoded data to a file.
func writePEMFile(path, pemType string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	return pem.Encode(file, &pem.Block{Type: pemType, Bytes: data})
}

// ParseCertificateFromPair parses the first x509.Certificate from a tls.Certificate.
// This is useful for testing purposes when you need to inspect certificate details.
func ParseCertificateFromPair(cert tls.Certificate) (*x509.Certificate, error) {
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("no certificates in pair")
	}
	return x509.ParseCertificate(cert.Certificate[0])
}
