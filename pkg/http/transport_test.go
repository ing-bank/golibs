package http

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

var rsaCertPEM = []byte(`-----BEGIN CERTIFICATE-----
MIIB0zCCAX2gAwIBAgIJAI/M7BYjwB+uMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANLJ
hPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wok/4xIA+ui35/MmNa
rtNuC+BdZ1tMuVCPFZcCAwEAAaNQME4wHQYDVR0OBBYEFJvKs8RfJaXTH08W+SGv
zQyKn0H8MB8GA1UdIwQYMBaAFJvKs8RfJaXTH08W+SGvzQyKn0H8MAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQEFBQADQQBJlffJHybjDGxRMqaRmDhX0+6v02TUKZsW
r5QuVbpQhH6u+0UgcW0jp9QwpxoPTLTWGXEWBBBurxFwiCBhkQ+V
-----END CERTIFICATE-----
`)

var rsaKeyPEM = []byte(testingKey(`-----BEGIN RSA TESTING KEY-----
MIIBOwIBAAJBANLJhPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wo
k/4xIA+ui35/MmNartNuC+BdZ1tMuVCPFZcCAwEAAQJAEJ2N+zsR0Xn8/Q6twa4G
6OB1M1WO+k+ztnX/1SvNeWu8D6GImtupLTYgjZcHufykj09jiHmjHx8u8ZZB/o1N
MQIhAPW+eyZo7ay3lMz1V01WVjNKK9QSn1MJlb06h/LuYv9FAiEA25WPedKgVyCW
SmUwbPw8fnTcpqDWE3yTO3vKcebqMSsCIBF3UmVue8YU3jybC3NxuXq3wNm34R8T
xVLHwDXh/6NJAiEAl2oHGGLz64BuAfjKrqwz7qMYr9HCLIe/YsoWq/olzScCIQDi
D2lWusoe2/nEqfDVVWGWlyJ7yOmqaVm/iNUN9B2N2g==
-----END RSA TESTING KEY-----
`))

func testingKey(s string) string { return strings.ReplaceAll(s, "TESTING KEY", "PRIVATE KEY") }

var testFiles = fstest.MapFS{
	"cert.pem": {
		Data:    []byte(rsaCertPEM),
		Mode:    0456,
		ModTime: time.Now(),
	},
	"key.pem": {
		Data:    []byte(rsaKeyPEM),
		Mode:    0456,
		ModTime: time.Now(),
	},
}

func TestInsecureSkipVerify(t *testing.T) {
	transport := &http.Transport{}
	opt := InsecureSkipVerify()
	if err := opt(transport); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be initialized, got nil")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify to be true, got false")
	}
}

func CreateTemp(t *testing.T, filename string, content []byte) *os.File {
	f, err := os.CreateTemp(t.TempDir(), filename)
	if err != nil {
		t.Fatalf("failed to create temp file %q: %v", filename, err)
	}
	_, err = f.Write(content)
	if err != nil {
		t.Fatalf("failed to write to temp file %q: %v", f.Name(), err)
	}
	return f
}

func TestWithTLS(t *testing.T) {
	transport := &http.Transport{}
	cert := CreateTemp(t, "cert.pem", rsaCertPEM)
	key := CreateTemp(t, "key.pem", rsaKeyPEM)

	opt := WithTLS(cert.Name(), key.Name())
	if err := opt(transport); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be set, got nil")
	}
	if len(transport.TLSClientConfig.Certificates) == 0 {
		t.Error("expected at least one certificate in TLSClientConfig")
	}
}
