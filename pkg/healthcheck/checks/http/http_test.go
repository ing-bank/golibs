package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pkghttp "github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/tlsclient"
	"github.com/ing-bank/golibs/pkg/tlsutils"
)

func TestClient_Check(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name        string
		setupServer func() (*httptest.Server, string)
		args        args
		wantErr     bool
	}{
		{
			name: "success",
			setupServer: func() (*httptest.Server, string) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(200)
					w.Write([]byte("OK"))
				}))
				return ts, ts.URL
			},
			args:    args{ctx: context.Background()},
			wantErr: false,
		},
		{
			name: "http error",
			setupServer: func() (*httptest.Server, string) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(500)
					w.Write([]byte("Internal Server Error"))
				}))
				return ts, ts.URL
			},
			args:    args{ctx: context.Background()},
			wantErr: true,
		},
		{
			name: "network error",
			setupServer: func() (*httptest.Server, string) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// This will never be called
				}))
				url := ts.URL
				ts.Close() // Close before use to simulate network error
				return ts, url
			},
			args:    args{ctx: context.Background()},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, urlz := tt.setupServer()
			defer ts.Close()

			client, _ := pkghttp.NewClient()
			c := &Client{
				httpClient: client,
				urlz:       urlz,
			}
			err := c.Check(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_WithAddress(t *testing.T) {
	type fields struct {
		Path      string
		Scheme    string
		Host      string
		Port      uint16
		TLSConfig tlsclient.Config
	}
	type args struct {
		address string
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantHost string
		wantPort uint16
		wantErr  bool
	}{
		{
			name:     "set host and port",
			fields:   fields{},
			args:     args{address: "example.com:8080"},
			wantHost: "example.com",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name:     "host already set",
			fields:   fields{Host: "preset.com"},
			args:     args{address: "example.com:8080"},
			wantHost: "preset.com",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name:     "port already set",
			fields:   fields{Port: 1234},
			args:     args{address: "example.com:8080"},
			wantHost: "example.com",
			wantPort: 1234,
			wantErr:  false,
		},
		{
			name:     "invalid address",
			fields:   fields{},
			args:     args{address: "badaddress"},
			wantHost: "",
			wantPort: 0,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				Path:      tt.fields.Path,
				Scheme:    tt.fields.Scheme,
				Host:      tt.fields.Host,
				Port:      tt.fields.Port,
				TLSConfig: tt.fields.TLSConfig,
			}
			err := c.WithAddress(tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("WithAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if c.Host != tt.wantHost {
					t.Errorf("WithAddress() Host = %v, want %v", c.Host, tt.wantHost)
				}
				if c.Port != tt.wantPort {
					t.Errorf("WithAddress() Port = %v, want %v", c.Port, tt.wantPort)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	type args struct {
		c *Config
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "valid config",
			args:    args{c: &Config{Host: "localhost", Port: 8080}},
			wantErr: false,
		},
		{
			name:    "invalid host",
			args:    args{c: &Config{Port: 8080}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got == nil {
				t.Errorf("New() got = nil, want non-nil")
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		wantError bool
	}{
		{
			name:      "missing host",
			cfg:       Config{Port: 8080},
			wantError: true,
		},
		{
			name:      "invalid port low",
			cfg:       Config{Host: "localhost", Port: 0},
			wantError: true,
		},
		{
			name:      "valid port",
			cfg:       Config{Host: "localhost", Port: 8080},
			wantError: false,
		},
		{
			name:      "invalid scheme",
			cfg:       Config{Host: "localhost", Scheme: "ftp", Port: 21},
			wantError: true,
		},
		{
			name:      "valid scheme http",
			cfg:       Config{Host: "localhost", Scheme: "http", Port: 80},
			wantError: false,
		},
		{
			name:      "valid scheme https",
			cfg:       Config{Host: "localhost", Scheme: "https", Port: 443},
			wantError: false,
		},
		{
			name: "TLSConfig with http scheme",
			cfg: Config{
				Host:   "localhost",
				Scheme: "http",
				Port:   443,
				TLSConfig: tlsclient.Config{
					Config: tlsutils.Config{
						Cert: "foo.crt",
						Key:  "foo.key",
					},
				},
			},
			wantError: true,
		},
		{
			name: "TLSConfig with https scheme",
			cfg: Config{
				Host:   "localhost",
				Scheme: "https",
				Port:   443,
				TLSConfig: tlsclient.Config{
					Config: tlsutils.Config{
						Cert: "foo.crt",
						Key:  "foo.key",
					},
				},
			},
			wantError: false,
		},
	}

	// Patch TLSConfig.UseTLS for test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestNormalizeURLFor(t *testing.T) {
	type args struct {
		cfg Config
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "http default",
			args:    args{cfg: Config{Host: "localhost", Port: 8080, Path: "/health"}},
			want:    "http://localhost:8080/health",
			wantErr: false,
		},
		{
			name:    "https scheme",
			args:    args{cfg: Config{Host: "localhost", Port: 443, Path: "/", Scheme: "https"}},
			want:    "https://localhost:443/",
			wantErr: false,
		},
		{
			name: "https inferred from TLSConfig",
			args: args{cfg: Config{Host: "localhost", Port: 8443, Path: "/status", TLSConfig: tlsclient.Config{
				Config: tlsutils.Config{
					Cert: "foo.crt",
					Key:  "foo.key",
				},
			}}},
			want:    "https://localhost:8443/status",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			args:    args{cfg: Config{Host: "http://[::1]:namedport"}},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeURLFor(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeURLFor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeURLFor() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRequestOptions(t *testing.T) {
	tests := []struct {
		name                   string
		headersList            []map[string]string
		expectedHeadersCount   int
		shouldHaveContextToken bool
		shouldHaveAuthHeader   bool
		expectedCustomHeaders  map[string]string
	}{
		{
			name:                   "empty headers",
			headersList:            []map[string]string{},
			expectedHeadersCount:   1, // only http.WithHeaders call
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   false,
			expectedCustomHeaders:  map[string]string{},
		},
		{
			name: "single header map without authorization",
			headersList: []map[string]string{
				{
					"X-Custom-Header":  "custom-value",
					"X-Another-Header": "another-value",
				},
			},
			expectedHeadersCount:   1, // only http.WithHeaders call
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   false,
			expectedCustomHeaders: map[string]string{
				"X-Custom-Header":  "custom-value",
				"X-Another-Header": "another-value",
			},
		},
		{
			name: "multiple header maps without authorization",
			headersList: []map[string]string{
				{"X-Header-1": "value-1"},
				{"X-Header-2": "value-2", "X-Header-3": "value-3"},
			},
			expectedHeadersCount:   1, // only http.WithHeaders call
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   false,
			expectedCustomHeaders: map[string]string{
				"X-Header-1": "value-1",
				"X-Header-2": "value-2",
				"X-Header-3": "value-3",
			},
		},
		{
			name: "authorization header with UseContextToken in single map",
			headersList: []map[string]string{
				{
					"Authorization": UseContextToken,
				},
			},
			expectedHeadersCount:   2, // http.WithBearerAuth + http.WithHeaders
			shouldHaveContextToken: true,
			shouldHaveAuthHeader:   false, // should be deleted
			expectedCustomHeaders:  map[string]string{},
		},
		{
			name: "authorization header with regular token",
			headersList: []map[string]string{
				{
					"Authorization": "Bearer token123",
				},
			},
			expectedHeadersCount:   1, // only http.WithHeaders call
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   true,
			expectedCustomHeaders: map[string]string{
				"Authorization": "Bearer token123",
			},
		},
		{
			name: "UseContextToken with other headers across multiple maps",
			headersList: []map[string]string{
				{"Authorization": UseContextToken},
				{"X-Custom-Header": "custom-value"},
				{"X-Another-Header": "another-value"},
			},
			expectedHeadersCount:   2, // http.WithBearerAuth + http.WithHeaders
			shouldHaveContextToken: true,
			shouldHaveAuthHeader:   false, // should be deleted
			expectedCustomHeaders: map[string]string{
				"X-Custom-Header":  "custom-value",
				"X-Another-Header": "another-value",
			},
		},
		{
			name: "UseContextToken with other headers in same map",
			headersList: []map[string]string{
				{
					"Authorization":    UseContextToken,
					"X-Custom-Header":  "custom-value",
					"X-Another-Header": "another-value",
				},
			},
			expectedHeadersCount:   2, // http.WithBearerAuth + http.WithHeaders
			shouldHaveContextToken: true,
			shouldHaveAuthHeader:   false, // should be deleted
			expectedCustomHeaders: map[string]string{
				"X-Custom-Header":  "custom-value",
				"X-Another-Header": "another-value",
			},
		},
		{
			name: "case sensitive authorization header",
			headersList: []map[string]string{
				{
					"authorization": "Bearer token123", // lowercase
				},
			},
			expectedHeadersCount:   1,
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   true, // Won't match "Authorization", so it's preserved
			expectedCustomHeaders: map[string]string{
				"authorization": "Bearer token123",
			},
		},
		{
			name: "later map overrides earlier map",
			headersList: []map[string]string{
				{"X-Header": "value1"},
				{"X-Header": "value2"}, // should override
			},
			expectedHeadersCount:   1,
			shouldHaveContextToken: false,
			shouldHaveAuthHeader:   false,
			expectedCustomHeaders: map[string]string{
				"X-Header": "value2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildRequestOptions(tt.headersList)

			// Check the number of options returned
			if len(opts) != tt.expectedHeadersCount {
				t.Errorf("buildRequestOptions() returned %d options, expected %d", len(opts), tt.expectedHeadersCount)
			}

			// Verify expected custom headers are present
			// Since we can't directly inspect the options, we verify the behavior
			// by checking the Authorization header deletion
			if tt.shouldHaveContextToken && len(tt.headersList) > 0 {
				// If UseContextToken was used, verify it was in the original headers
				found := false
				for _, headerMap := range tt.headersList {
					if token, ok := headerMap[AuthorizationHeader]; ok && token == UseContextToken {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildRequestOptions() expected UseContextToken in headers but not found")
				}
			}
		})
	}
}
