package tlsutils

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUseTLS(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name:     "returns false when config is nil",
			config:   nil,
			expected: false,
		},
		{
			name: "returns false when cert is empty",
			config: &Config{
				Cert: "",
				Key:  "key.pem",
			},
			expected: false,
		},
		{
			name: "returns false when key is empty",
			config: &Config{
				Cert: "cert.pem",
				Key:  "",
			},
			expected: false,
		},
		{
			name: "returns false when both cert and key are empty",
			config: &Config{
				Cert: "",
				Key:  "",
			},
			expected: false,
		},
		{
			name: "returns true when cert and key are both set",
			config: &Config{
				Cert: "cert.pem",
				Key:  "key.pem",
			},
			expected: true,
		},
		{
			name: "returns false when disabled is true even with cert and key set",
			config: &Config{
				Cert:     "cert.pem",
				Key:      "key.pem",
				Disabled: true,
			},
			expected: false,
		},
		{
			name: "returns true when disabled is false and cert and key are set",
			config: &Config{
				Cert:     "cert.pem",
				Key:      "key.pem",
				Disabled: false,
			},
			expected: true,
		},
		{
			name: "returns true with rootCAs set along with cert and key",
			config: &Config{
				Cert:    "cert.pem",
				Key:     "key.pem",
				RootCAs: []string{"ca.pem"},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.config.UseTLS())
		})
	}
}

func TestNewConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cert            string
		key             string
		cacerts         []string
		expectedCert    string
		expectedKey     string
		expectedRootCAs []string
	}{
		{
			name:            "creates config with cert and key",
			cert:            "cert.pem",
			key:             "key.pem",
			cacerts:         nil,
			expectedCert:    "cert.pem",
			expectedKey:     "key.pem",
			expectedRootCAs: nil,
		},
		{
			name:            "creates config with cert, key, and one CA",
			cert:            "cert.pem",
			key:             "key.pem",
			cacerts:         []string{"ca.pem"},
			expectedCert:    "cert.pem",
			expectedKey:     "key.pem",
			expectedRootCAs: []string{"ca.pem"},
		},
		{
			name:            "creates config with cert, key, and multiple CAs",
			cert:            "cert.pem",
			key:             "key.pem",
			cacerts:         []string{"ca1.pem", "ca2.pem", "ca3.pem"},
			expectedCert:    "cert.pem",
			expectedKey:     "key.pem",
			expectedRootCAs: []string{"ca1.pem", "ca2.pem", "ca3.pem"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := NewConfig(tc.cert, tc.key, tc.cacerts...)
			assert.Equal(t, tc.expectedCert, c.Cert)
			assert.Equal(t, tc.expectedKey, c.Key)
			assert.Equal(t, tc.expectedRootCAs, c.RootCAs)
			assert.Equal(t, tls.VersionName(DefaultMinVersion), c.MinVersion)
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	c := DefaultConfig()
	assert.NotNil(t, c)
	assert.Equal(t, tls.VersionName(DefaultMinVersion), c.MinVersion)
	assert.Empty(t, c.Cert)
	assert.Empty(t, c.Key)
	assert.Empty(t, c.RootCAs)
	assert.False(t, c.Disabled)
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		config             *Config
		expectedMinVersion string
	}{
		{
			name:               "sets MinVersion when empty",
			config:             &Config{},
			expectedMinVersion: tls.VersionName(DefaultMinVersion),
		},
		{
			name: "does not override MinVersion when set",
			config: &Config{
				MinVersion: "TLS 1.3",
			},
			expectedMinVersion: "TLS 1.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.config.ApplyDefaults()
			assert.Equal(t, tc.expectedMinVersion, tc.config.MinVersion)
		})
	}
}

func TestParseClientAuthType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expected    tls.ClientAuthType
		expectError bool
	}{
		{
			name:     "NoClientCert",
			input:    TLSNoClientCert,
			expected: tls.NoClientCert,
		},
		{
			name:     "RequestClientCert",
			input:    TLSRequestClientCert,
			expected: tls.RequestClientCert,
		},
		{
			name:     "RequireAnyClientCert",
			input:    TLSRequireAnyClientCert,
			expected: tls.RequireAnyClientCert,
		},
		{
			name:     "VerifyClientCertIfGiven",
			input:    TLSVerifyClientCertIfGiven,
			expected: tls.VerifyClientCertIfGiven,
		},
		{
			name:     "RequireAndVerifyClientCert",
			input:    TLSRequireAndVerifyClientCert,
			expected: tls.RequireAndVerifyClientCert,
		},
		{
			name:        "unknown type returns error",
			input:       "InvalidType",
			expectError: true,
		},
		{
			name:        "empty string returns error",
			input:       "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseClientAuthType(tc.input)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestParseTLSVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected uint16
	}{
		{
			name:     "TLS 1.0",
			input:    TLSVersion10,
			expected: tls.VersionTLS10,
		},
		{
			name:     "TLS 1.1",
			input:    TLSVersion11,
			expected: tls.VersionTLS11,
		},
		{
			name:     "TLS 1.2",
			input:    TLSVersion12,
			expected: tls.VersionTLS12,
		},
		{
			name:     "TLS 1.3",
			input:    TLSVersion13,
			expected: tls.VersionTLS13,
		},
		{
			name:     "unknown version returns 0",
			input:    "TLS 2.0",
			expected: 0,
		},
		{
			name:     "empty string returns 0",
			input:    "",
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := ParseTLSVersion(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}
