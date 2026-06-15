package certificate

import (
	"os"

	"github.com/spf13/pflag"
)

const (
	DefaultCertificateEnabled  = false
	FlagCertificateAuthEnabled = "middleware-certificate-auth-enabled"
)

func init() {
	if os.Getenv("PFLAGS_CERTIFICATE_AUTH_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	flags.Bool(FlagCertificateAuthEnabled, DefaultCertificateEnabled, "Enable certificate based authentication middleware")
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if fs.Changed(FlagCertificateAuthEnabled) {
		if c.Enabled, err = fs.GetBool(FlagCertificateAuthEnabled); err != nil {
			return err
		}
	}
	return nil
}
