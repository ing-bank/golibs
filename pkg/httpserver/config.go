package httpserver

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultHost              = ""
	DefaultPort              = 8089
	DefaultReadTimeout       = 10 * time.Second
	DefaultReadHeaderTimeout = 10 * time.Second
	DefaultWriteTimeout      = 10 * time.Second
	DefaultShutdownTimeout   = 30 * time.Second
	DefaultMaxHeaderBytes    = http.DefaultMaxHeaderBytes // 1 MB

	FlagHTTPHost              = "http-host"
	FlagHTTPPort              = "http-port"
	FlagHTTPReadTimeout       = "http-read-timeout"
	FlagHTTPReadHeaderTimeout = "http-read-header-timeout"
	FlagHTTPWriteTimeout      = "http-write-timeout"
	FlagHTTPShutdownTimeout   = "http-shutdown-timeout"
	FlagHTTPMaxHeaderBytes    = "http-max-header-bytes"
) //nolint:gosec // header timeout flags are not credentials

type Config struct {
	Host              string            `json:"host" yaml:"host"`
	Port              uint16            `json:"port" yaml:"port"`
	ReadTimeout       metav1.Duration   `json:"readTimeout" yaml:"readTimeout"`
	ReadHeaderTimeout metav1.Duration   `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`
	WriteTimeout      metav1.Duration   `json:"writeTimeout" yaml:"writeTimeout"`
	ShutdownTimeout   metav1.Duration   `json:"shutdownTimeout" yaml:"shutdownTimeout"`
	MaxHeaderBytes    int               `json:"maxHeaderBytes" yaml:"maxHeaderBytes"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func init() {
	if os.Getenv("PFLAGS_HTTPSERVER_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()
	flags.String(FlagHTTPHost, c.Host, "HTTP server host")
	flags.Uint16(FlagHTTPPort, c.Port, "Server port.")
	flags.Duration(FlagHTTPReadTimeout, c.ReadTimeout.Duration, "HTTP server read timeout")
	flags.Duration(FlagHTTPReadHeaderTimeout, c.ReadHeaderTimeout.Duration, "HTTP server read header timeout")
	flags.Duration(FlagHTTPWriteTimeout, c.WriteTimeout.Duration, "HTTP server write timeout")
	flags.Duration(FlagHTTPShutdownTimeout, c.ShutdownTimeout.Duration, "HTTP server shutdown timeout")
	flags.Int(FlagHTTPMaxHeaderBytes, c.MaxHeaderBytes, "HTTP server max header bytes")
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}

	var err error
	if fs.Changed(FlagHTTPHost) {
		if c.Host, err = fs.GetString(FlagHTTPHost); err != nil {
			return err
		}
	}

	if fs.Changed(FlagHTTPPort) {
		if c.Port, err = fs.GetUint16(FlagHTTPPort); err != nil {
			return err
		}
	}

	if fs.Changed(FlagHTTPReadTimeout) {
		d, err := fs.GetDuration(FlagHTTPReadTimeout)
		if err != nil {
			return err
		}
		c.ReadTimeout = metav1.Duration{Duration: d}
	}

	if fs.Changed(FlagHTTPReadHeaderTimeout) {
		d, err := fs.GetDuration(FlagHTTPReadHeaderTimeout)
		if err != nil {
			return err
		}
		c.ReadHeaderTimeout = metav1.Duration{Duration: d}
	}

	if fs.Changed(FlagHTTPWriteTimeout) {
		d, err := fs.GetDuration(FlagHTTPWriteTimeout)
		if err != nil {
			return err
		}
		c.WriteTimeout = metav1.Duration{Duration: d}
	}

	if fs.Changed(FlagHTTPShutdownTimeout) {
		d, err := fs.GetDuration(FlagHTTPShutdownTimeout)
		if err != nil {
			return err
		}
		c.ShutdownTimeout = metav1.Duration{Duration: d}
	}

	if fs.Changed(FlagHTTPMaxHeaderBytes) {
		if c.MaxHeaderBytes, err = fs.GetInt(FlagHTTPMaxHeaderBytes); err != nil {
			return err
		}
	}
	return nil
}

func ApplyDefaultConfig(cfg *Config) {
	cfg.ApplyDefaults()
}

func (c *Config) ApplyDefaults() {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.ReadTimeout.Duration == 0 {
		c.ReadTimeout = metav1.Duration{Duration: DefaultReadTimeout}
	}
	if c.ReadHeaderTimeout.Duration == 0 {
		c.ReadHeaderTimeout = metav1.Duration{Duration: DefaultReadHeaderTimeout}
	}
	if c.WriteTimeout.Duration == 0 {
		c.WriteTimeout = metav1.Duration{Duration: DefaultWriteTimeout}
	}
	if c.ShutdownTimeout.Duration == 0 {
		c.ShutdownTimeout = metav1.Duration{Duration: DefaultShutdownTimeout}
	}
	if c.MaxHeaderBytes == 0 {
		c.MaxHeaderBytes = DefaultMaxHeaderBytes
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("http server config is nil")
	}
	if c.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	if c.ReadTimeout.Duration < 0 {
		return fmt.Errorf("read timeout must be non-negative")
	}
	if c.ReadHeaderTimeout.Duration < 0 {
		return fmt.Errorf("read header timeout must be non-negative")
	}
	if c.WriteTimeout.Duration < 0 {
		return fmt.Errorf("write timeout must be non-negative")
	}
	if c.ShutdownTimeout.Duration < 0 {
		return fmt.Errorf("shutdown timeout must be non-negative")
	}
	if c.MaxHeaderBytes <= 0 {
		return fmt.Errorf("max header bytes must be positive")
	}
	return nil
}
