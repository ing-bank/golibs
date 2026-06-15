package tlsserver

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/spf13/pflag"
)

const (
	DefaultClientAuth = tls.NoClientCert
	FlagTLSCert       = "tls-server-cert"
	FlagTLSKey        = "tls-server-key"
	FlagTLSRootCAs    = "tls-server-cacerts"
	FlagTLSMinVersion = "tls-server-min-version"
	FlagTLSClientAuth = "tls-client-auth"
)

type Config struct {
	tlsutils.Config `json:",inline" yaml:",inline"`
	ClientAuthType  string `yaml:"clientAuthType" json:"clientAuthType"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func init() {
	if os.Getenv("PFLAGS_TLSSERVER_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()
	clientAuthTypeUsage := fmt.Sprintf("Client authentication type. One of: %s, %s, %s, %s, %s",
		tlsutils.TLSNoClientCert,
		tlsutils.TLSRequestClientCert,
		tlsutils.TLSRequireAnyClientCert,
		tlsutils.TLSVerifyClientCertIfGiven,
		tlsutils.TLSRequireAndVerifyClientCert)
	flags.String(FlagTLSClientAuth, c.ClientAuthType, clientAuthTypeUsage)
	flags.String(FlagTLSCert, c.Cert, "Path to the TLS certificate file")
	flags.String(FlagTLSKey, c.Key, "Path to the TLS key file")
	flags.StringSlice(FlagTLSRootCAs, c.RootCAs, "Paths to the CA certificate files")
	flags.String(FlagTLSMinVersion, c.MinVersion, "Minimum TLS version (e.g., VersionTLS12, VersionTLS13)")
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}

	var err error
	if fs.Changed(FlagTLSCert) {
		c.Cert, err = fs.GetString(FlagTLSCert)
		if err != nil {
			return err
		}
	}
	if fs.Changed(FlagTLSKey) {
		c.Key, err = fs.GetString(FlagTLSKey)
		if err != nil {
			return err
		}
	}
	if fs.Changed(FlagTLSRootCAs) {
		c.RootCAs, err = fs.GetStringSlice(FlagTLSRootCAs)
		if err != nil {
			return err
		}
	}
	if fs.Changed(FlagTLSMinVersion) {
		c.MinVersion, err = fs.GetString(FlagTLSMinVersion)
		if err != nil {
			return err
		}
	}
	if fs.Changed(FlagTLSClientAuth) {
		c.ClientAuthType, err = fs.GetString(FlagTLSClientAuth)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) ParseClientAuthType() (tls.ClientAuthType, error) {
	return tlsutils.ParseClientAuthType(c.ClientAuthType)
}

func (c *Config) ApplyDefaults() {
	c.Config.ApplyDefaults()
	if c.ClientAuthType == "" {
		c.ClientAuthType = DefaultClientAuth.String()
	}
}

func (c *Config) Validate() error {
	_, err := tlsutils.ParseClientAuthType(c.ClientAuthType)
	if err != nil {
		return err
	}
	if c.Cert == "" {
		return fmt.Errorf("tls-cert must be provided")
	}
	if c.Key == "" {
		return fmt.Errorf("tls-key must be provided")
	}
	if c.MinVersion != "" {
		_, err = tlsutils.ParseTLSVersion(c.MinVersion)
		if err != nil {
			return fmt.Errorf("invalid tls-min-version: %w", err)
		}
	}
	if c.ClientAuthType == tlsutils.TLSRequireAndVerifyClientCert {
		if len(c.RootCAs) == 0 {
			return fmt.Errorf("at least one CA certificate must be provided in RootCAs when ClientAuthType is RequireAndVerifyClientCert")
		}
	}
	return nil
}
