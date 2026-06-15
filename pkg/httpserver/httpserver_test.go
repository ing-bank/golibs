package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/tlsserver"
)

func TestWithHandler(t *testing.T) {
	s := DefaultServerConfig()
	s.withHandler(nil)
	if s.Server.Handler == nil {
		t.Error("Handler should not be nil after WithHandler(nil)")
	}
	h := http.NewServeMux()
	s.withHandler(h)
	if s.Server.Handler != h {
		t.Error("Handler not set correctly")
	}
}

func TestNewAndNewForConfig(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if s == nil || s.Server == nil {
		t.Error("Server or http.Server is nil")
	}
}

func TestApplyDefaults(t *testing.T) {
	s := &Server{Server: &http.Server{}}
	s.ApplyDefaults()
	if s.Server.Addr == "" || s.Server.ReadTimeout <= 0 || s.shutdownTimeout <= 0 {
		t.Error("Defaults not applied correctly")
	}
}

func TestValidate(t *testing.T) {
	s := DefaultServerConfig()
	s.Server.Handler = http.NewServeMux()
	s.Server.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if err := s.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestRegisterRoutes(t *testing.T) {
	engine := gin.New()
	s := DefaultServerConfig()
	s.Server.Handler = engine
	h := http.NewServeMux()
	s.RegisterRoutes(h)
	// Gin middlewares are not exported, but we can check that Use does not panic
}

func TestRunBackgroundAndWait(t *testing.T) {
	s := DefaultServerConfig()
	s.Server.Handler = http.NewServeMux()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	errCh := s.RunBackground(ctx)
	cancel()
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for server shutdown")
	}
	if err := s.Wait(); err != nil {
		t.Errorf("Wait returned error: %v", err)
	}
}

func TestRunFunction(t *testing.T) {
	handler := http.NewServeMux()
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err := Run(ctx, handler)
	if err != nil {
		t.Errorf("Run returned error: %v", err)
	}
}

func createTempCertAndKeyFiles(t *testing.T, certData, keyData string) (certPath, keyPath string) {
	tmpdir := t.TempDir()

	certFile, err := os.CreateTemp(tmpdir, "cert.pem")
	if err != nil {
		t.Fatalf("Failed to create temp cert file: %v", err)
	}
	defer func() {
		if cerr := certFile.Close(); cerr != nil {
			t.Errorf("Failed to close cert file: %v", cerr)
		}
	}()
	if _, err := certFile.Write([]byte(certData)); err != nil {
		t.Fatalf("Failed to write cert to temp file: %v", err)
	}
	certPath = certFile.Name()

	keyFile, err := os.CreateTemp(tmpdir, "key.pem")
	if err != nil {
		t.Fatalf("Failed to create temp key file: %v", err)
	}
	defer func() {
		if kerr := keyFile.Close(); kerr != nil {
			t.Errorf("Failed to close key file: %v", kerr)
		}
	}()
	if _, err := keyFile.Write([]byte(keyData)); err != nil {
		t.Fatalf("Failed to write key to temp file: %v", err)
	}
	keyPath = keyFile.Name()

	return certPath, keyPath
}

func TestRunTLSFunction(t *testing.T) {
	handler := http.NewServeMux()

	certPath, keyPath := createTempCertAndKeyFiles(t, testCert, testKey)
	tlsconfig, err := tlsserver.New(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to create TLS config: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	err = RunTLS(ctx, handler, tlsconfig)
	if err != nil && err != http.ErrServerClosed {
		t.Errorf("RunTLS returned error: %v", err)
	}
}

func testingKey(s string) string { return strings.ReplaceAll(s, "TESTING KEY", "PRIVATE KEY") }

var (
	testCert = `-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----`

	testKey = testingKey(`-----BEGIN RSA TESTING KEY-----
MIIBOwIBAAJBANLJhPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wo
k/4xIA+ui35/MmNartNuC+BdZ1tMuVCPFZcCAwEAAQJAEJ2N+zsR0Xn8/Q6twa4G
6OB1M1WO+k+ztnX/1SvNeWu8D6GImtupLTYgjZcHufykj09jiHmjHx8u8ZZB/o1N
MQIhAPW+eyZo7ay3lMz1V01WVjNKK9QSn1MJlb06h/LuYv9FAiEA25WPedKgVyCW
SmUwbPw8fnTcpqDWE3yTO3vKcebqMSsCIBF3UmVue8YU3jybC3NxuXq3wNm34R8T
xVLHwDXh/6NJAiEAl2oHGGLz64BuAfjKrqwz7qMYr9HCLIe/YsoWq/olzScCIQDi
D2lWusoe2/nEqfDVVWGWlyJ7yOmqaVm/iNUN9B2N2g==
-----END RSA TESTING KEY-----
`)
)

func TestServer_RunBackground(t *testing.T) {
	tests := []struct {
		name    string
		handler http.Handler
		wantErr bool
	}{
		{
			name:    "nil handler",
			handler: nil,
			wantErr: true,
		},
		{
			name:    "valid handler",
			handler: http.NewServeMux(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(WithHandler(tt.handler))
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("New() failed: %v", err)
				}
				// If handler is nil, New() may fail, so skip rest
				return
			}
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			errCh := s.RunBackground(ctx)

			go func() {
				time.Sleep(1 * time.Second)
				if err := s.Shutdown(ctx); err != nil {
					t.Errorf("Shutdown() unexpected error: %v", err)
				}
			}()

			select {
			case err := <-errCh:
				if tt.wantErr && err == nil {
					t.Errorf("RunBackground() expected error, got nil")
				}
				if !tt.wantErr && err != nil && err != http.ErrServerClosed {
					t.Errorf("RunBackground() unexpected error: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Error("RunBackground() timeout waiting for result")
			}

			if err := s.Wait(); err != nil && err != http.ErrServerClosed {
				t.Errorf("Wait() unexpected error: %v", err)
			}
		})
	}
}

func TestServer_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		server *Server
		errMsg string
	}{
		{
			name:   "nil Server",
			server: &Server{Server: nil, shutdownTimeout: time.Second},
			errMsg: "httpServer cannot be nil",
		},
		{
			name:   "invalid shutdownTimeout",
			server: &Server{Server: &http.Server{}, shutdownTimeout: 0},
			errMsg: "shutdownTimeout must be a positive duration",
		},
		{
			name:   "empty Addr",
			server: &Server{Server: &http.Server{Addr: ""}, shutdownTimeout: time.Second},
			errMsg: "httpServer address cannot be empty",
		},
		{
			name:   "invalid ReadHeaderTimeout",
			server: &Server{Server: &http.Server{Addr: "x", ReadHeaderTimeout: 0}, shutdownTimeout: time.Second},
			errMsg: "httpServer ReadHeaderTimeout must be a positive duration",
		},
		{
			name:   "invalid WriteTimeout",
			server: &Server{Server: &http.Server{Addr: "x", ReadHeaderTimeout: time.Second, WriteTimeout: 0}, shutdownTimeout: time.Second},
			errMsg: "httpServer WriteTimeout must be a positive duration",
		},
		{
			name:   "invalid MaxHeaderBytes",
			server: &Server{Server: &http.Server{Addr: "x", ReadHeaderTimeout: time.Second, WriteTimeout: time.Second, MaxHeaderBytes: 0}, shutdownTimeout: time.Second},
			errMsg: "httpServer MaxHeaderBytes must be a positive integer",
		},
		{
			name:   "TLSConfig.MinVersion too low",
			server: &Server{Server: &http.Server{Addr: "x", ReadHeaderTimeout: time.Second, WriteTimeout: time.Second, MaxHeaderBytes: 1, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS10}}, shutdownTimeout: time.Second},
			errMsg: "minimum version of TLS 1.2",
		},
		{
			name:   "valid config",
			server: &Server{Server: &http.Server{Addr: "x", ReadHeaderTimeout: time.Second, WriteTimeout: time.Second, MaxHeaderBytes: 1, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12}}, shutdownTimeout: time.Second},
			errMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.server.Validate()
			if tt.errMsg == "" {
				if err != nil {
					t.Errorf("expected nil for valid config, got: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got: %v", tt.errMsg, err)
				}
			}
		})
	}
}
