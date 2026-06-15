package ginserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/ginserver/proxy"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/certificate"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/npa"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/oauth"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/user"
	"github.com/ing-bank/golibs/pkg/middleware/gzip"
	"github.com/ing-bank/golibs/pkg/middleware/logger"
	"github.com/ing-bank/golibs/pkg/middleware/metrics"
	"github.com/ing-bank/golibs/pkg/middleware/requestid"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/reloader"
	"github.com/ing-bank/golibs/pkg/tlsserver"
	"github.com/ing-bank/golibs/pkg/tlsutils"
	"github.com/ing-bank/golibs/pkg/trace"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type GinMode string

const (
	ModeDebug   GinMode = gin.DebugMode
	ModeRelease GinMode = gin.ReleaseMode
	ModeTest    GinMode = gin.TestMode

	DefaultMetricsPath = "/metrics"

	DefaultMode                   GinMode = ModeRelease
	DefaultPProfEnabled                   = false
	DefaultHealthcheckEnabled             = true
	DefaultReloaderEnabled                = false
	DefaultProxyEnabled                   = false
	DefaultRecoveryEnabled                = true
	DefaultMetricsEnabled                 = true
	DefaultLoggerEnabled                  = true
	DefaultRequestIDEnabled               = true
	DefaultGZIPEnabled                    = false
	DefaultUserAuthEnabled                = false
	DefaultNPAAuthEnabled                 = false
	DefaultCertificateAuthEnabled         = false
	DefaultOAuthEnabled                   = false
	DefaultTraceEnabled                   = false

	FlagGinMode = "gin-mode"

	FlagRecoveryEnabled            = "middleware-recovery-enabled"
	FlagMiddlewareLoggerEnabled    = "middleware-logger-enabled"
	FlagMiddlewareMetricsEnabled   = "middleware-metrics-enabled"
	FlagMiddlewareRequestIDEnabled = "middleware-requestid-enabled"
	FlagMiddlewareGZIPEnabled      = "middleware-gzip-enabled"
	FlagMiddlewareGZIPCompression  = "middleware-gzip-compression"
	FlagMiddlewareUserAuthEnabled  = "middleware-user-auth-enabled"
	FlagMiddlewareOAuthEnabled     = "middleware-oauth-enabled"
	FlagMiddlewareNPAAuthEnabled   = "middleware-npa-auth-enabled"
	FlagMiddlewareTraceEnabled     = "middleware-trace-enabled"

	FlagServicePProfEnabled               = "internal-service-pprof-enabled"
	FlagServiceMetricsEnabled             = "internal-service-metrics-enabled"
	FlagServiceHealthcheckEnabled         = "internal-service-healthcheck-enabled"
	FlagServiceCertificateReloaderEnabled = "internal-service-certificate-reloader-enabled"
)

type Config struct {
	Mode          GinMode           `yaml:"mode" json:"mode"`
	TLSConfig     tlsserver.Config  `yaml:"tls" json:"tls"`
	HTTPServer    httpserver.Config `yaml:"http" json:"http"`
	Middleware    MiddlewareConfig  `yaml:"middleware" json:"middleware"`
	ServiceConfig ServiceConfig     `yaml:"internalServices" json:"internalServices"`
}

type ServiceConfig struct {
	PProfConfig  *PProfConfig       `yaml:"pprof" json:"pprof"`
	MetricConfig *MetricConfig      `yaml:"metrics" json:"metrics"`
	Healthcheck  *HealthCheckConfig `yaml:"healthcheck" json:"healthcheck"`
	Reloader     *ReloaderConfig    `yaml:"reloader" json:"reloader"`
	ProxyConfig  *ProxyConfig       `yaml:"proxy" json:"proxy"`
}

type ProxyConfig struct {
	Enabled      bool `yaml:"enabled" json:"enabled"`
	proxy.Config `yaml:",inline" json:",inline"`
}

type ReloaderConfig struct {
	Enabled         bool `yaml:"enabled" json:"enabled"`
	reloader.Config `yaml:",inline" json:",inline"`
}

type HealthCheckConfig struct {
	Enabled            bool `yaml:"enabled" json:"enabled"`
	healthcheck.Config `yaml:",inline" json:",inline"`
}

type MetricConfig struct {
	Path    string `yaml:"path" json:"path"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}

type PProfConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type MiddlewareConfig struct {
	RecoveryConfig        *RecoveryConfig     `yaml:"recovery" json:"recovery"`
	MetricsConfig         *metrics.Config     `yaml:"metrics" json:"metrics"`
	LoggerConfig          *logger.Config      `yaml:"logging" json:"logging"`
	TraceConfig           *TraceConfig        `yaml:"tracing" json:"tracing"`
	RequestIDConfig       *requestid.Config   `yaml:"requestID" json:"requestID"`
	GZIPConfig            *gzip.Config        `yaml:"gzip" json:"gzip"`
	OauthConfig           *oauth.Config       `yaml:"oauth" json:"oauth"`
	UserAuthConfig        *user.Config        `yaml:"userAuth" json:"userAuth"`
	NPAAuthConfig         *npa.Config         `yaml:"npaAuth" json:"npaAuth"`
	CertificateAuthConfig *certificate.Config `yaml:"certificateAuth" json:"certificateAuth"`
}

type RecoveryConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// MiddlewareOption holds options for different middleware types
type MiddlewareOption struct {
	OAuth []oauth.Option
}

// NewMiddlewareForConfig creates Gin middleware handlers based on the provided MiddlewareConfig and options
func NewMiddlewareForConfig(c *MiddlewareConfig, opts ...MiddlewareOption) ([]gin.HandlerFunc, error) {
	if c == nil {
		return nil, fmt.Errorf("middleware config is nil")
	}
	cfg := *c // shadow copy

	middlewareOpts := opt.Opt(MiddlewareOption{}, opts)

	var handlers []gin.HandlerFunc

	if cfg.RecoveryConfig != nil && cfg.RecoveryConfig.Enabled {
		handlers = append(handlers, gin.Recovery())
	}
	if cfg.TraceConfig != nil && cfg.TraceConfig.Enabled {
		handlers = append(handlers, otelgin.Middleware(cfg.TraceConfig.ServiceName, otelgin.WithFilter(trace.SkipPaths(cfg.TraceConfig.SkipPaths))))
	}
	if cfg.RequestIDConfig != nil && cfg.RequestIDConfig.Enabled {
		handlers = append(handlers, requestid.Middleware(cfg.RequestIDConfig))
	}
	if cfg.MetricsConfig != nil && cfg.MetricsConfig.Enabled {
		handlers = append(handlers, metrics.Middleware(cfg.MetricsConfig))
	}
	if cfg.LoggerConfig != nil && cfg.LoggerConfig.Enabled {
		handlers = append(handlers, logger.Middleware(cfg.LoggerConfig))
	}
	if cfg.GZIPConfig != nil && cfg.GZIPConfig.Enabled {
		handlers = append(handlers, gzip.Middleware(cfg.GZIPConfig))
	}
	if cfg.CertificateAuthConfig != nil && cfg.CertificateAuthConfig.Enabled {
		handlers = append(handlers, certificate.Middleware(*cfg.CertificateAuthConfig))
	}
	if cfg.UserAuthConfig != nil && cfg.UserAuthConfig.Enabled {
		handlers = append(handlers, user.Middleware(cfg.UserAuthConfig))
	}
	if cfg.NPAAuthConfig != nil && cfg.NPAAuthConfig.Enabled {
		handlers = append(handlers, npa.Middleware(*cfg.NPAAuthConfig))
	}
	if cfg.OauthConfig != nil && cfg.OauthConfig.Enabled {
		handlers = append(handlers, oauth.Middleware(*cfg.OauthConfig, middlewareOpts.OAuth...))
	}
	return handlers, nil
}

type TraceConfig struct {
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	ServiceName string   `yaml:"serviceName" json:"serviceName"`
	SkipPaths   []string `yaml:"skipPaths,omitempty" json:"skipPaths,omitempty"`
}

func DefaultConfig() *Config {
	c := new(Config)
	c.ApplyDefaults()
	return c
}

func init() {
	if os.Getenv("PFLAGS_GINSERVER_ENABLED") == "1" {
		RegisterFlags(pflag.CommandLine)
	}
}

func RegisterFlags(flags *pflag.FlagSet) {
	if flags == nil {
		flags = pflag.CommandLine
	}
	c := DefaultConfig()

	httpserver.RegisterFlags(flags)
	tlsserver.RegisterFlags(flags)
	certificate.RegisterFlags(flags)

	flags.String(FlagGinMode, string(c.Mode), "Gin mode (debug, release, test)")

	flags.Bool(FlagServicePProfEnabled, c.ServiceConfig.PProfConfig != nil && c.ServiceConfig.PProfConfig.Enabled, "Enable pprof route")
	flags.Bool(FlagServiceMetricsEnabled, c.ServiceConfig.MetricConfig != nil && c.ServiceConfig.MetricConfig.Enabled, "Enable internal metrics route")
	flags.Bool(FlagServiceHealthcheckEnabled, c.ServiceConfig.Healthcheck != nil && c.ServiceConfig.Healthcheck.Enabled, "Enable internal healthcheck route")
	flags.Bool(FlagServiceCertificateReloaderEnabled, c.ServiceConfig.Reloader != nil && c.ServiceConfig.Reloader.Enabled, "Enable internal certificate reloader")

	flags.Bool(FlagRecoveryEnabled, c.Middleware.RecoveryConfig != nil && c.Middleware.RecoveryConfig.Enabled, "Enable recovery middleware")
	flags.Bool(FlagMiddlewareMetricsEnabled, c.Middleware.MetricsConfig != nil && c.Middleware.MetricsConfig.Enabled, "Enable metrics middleware")
	flags.Bool(FlagMiddlewareLoggerEnabled, c.Middleware.LoggerConfig != nil && c.Middleware.LoggerConfig.Enabled, "Enable logger middleware")
	flags.Bool(FlagMiddlewareRequestIDEnabled, c.Middleware.RequestIDConfig != nil && c.Middleware.RequestIDConfig.Enabled, "Enable request ID middleware")
	flags.Bool(FlagMiddlewareGZIPEnabled, c.Middleware.GZIPConfig != nil && c.Middleware.GZIPConfig.Enabled, "Enable gzip middleware")

	var compressionMode = int(gzip.DefaultCompression)
	if c.Middleware.GZIPConfig != nil {
		compressionMode = int(c.Middleware.GZIPConfig.CompressionMode)
	}
	flags.Int(FlagMiddlewareGZIPCompression, compressionMode, "GZIP compression level: 0=NoCompression, 1=BestSpeed, 2=BestCompression, 3=DefaultCompression, 4=HuffmanOnly")

	flags.Bool(FlagMiddlewareUserAuthEnabled, c.Middleware.UserAuthConfig != nil && c.Middleware.UserAuthConfig.Enabled, "Enable user middleware")
	flags.Bool(FlagMiddlewareOAuthEnabled, c.Middleware.OauthConfig != nil && c.Middleware.OauthConfig.Enabled, "Enable oauth middleware")
	flags.Bool(FlagMiddlewareNPAAuthEnabled, c.Middleware.NPAAuthConfig != nil && c.Middleware.NPAAuthConfig.Enabled, "Enable NPA middleware")
	flags.Bool(FlagMiddlewareTraceEnabled, c.Middleware.TraceConfig != nil && c.Middleware.TraceConfig.Enabled, "Enable trace middleware")
}

func ApplyDefaultConfig(cfg *Config) {
	cfg.ApplyDefaults()
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	mode := fs.Lookup(FlagGinMode)
	if mode == nil {
		return fmt.Errorf("flag %s not found", FlagGinMode)
	}
	if fs.Changed(FlagGinMode) {
		c.Mode = GinMode(mode.Value.String())
	}

	if err = c.HTTPServer.BindFlags(fs); err != nil {
		return fmt.Errorf("httpserver: %w", err)
	}
	if err = c.TLSConfig.BindFlags(fs); err != nil {
		return fmt.Errorf("tlsserver: %w", err)
	}
	if err = c.Middleware.BindFlags(fs); err != nil {
		return fmt.Errorf("middleware: %w", err)
	}
	if err = c.ServiceConfig.BindFlags(fs); err != nil {
		return fmt.Errorf("serviceConfig: %w", err)
	}
	return nil
}

func (s *ServiceConfig) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if s.PProfConfig != nil && fs.Changed(FlagServicePProfEnabled) {
		if s.PProfConfig.Enabled, err = fs.GetBool(FlagServicePProfEnabled); err != nil {
			return fmt.Errorf("pprof: %w", err)
		}
	}
	if s.MetricConfig != nil && fs.Changed(FlagServiceMetricsEnabled) {
		if s.MetricConfig.Enabled, err = fs.GetBool(FlagServiceMetricsEnabled); err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
	}
	if s.Healthcheck != nil && fs.Changed(FlagServiceHealthcheckEnabled) {
		if s.Healthcheck.Enabled, err = fs.GetBool(FlagServiceHealthcheckEnabled); err != nil {
			return fmt.Errorf("healthcheck: %w", err)
		}
	}
	if s.Reloader != nil && fs.Changed(FlagServiceCertificateReloaderEnabled) {
		if s.Reloader.Enabled, err = fs.GetBool(FlagServiceCertificateReloaderEnabled); err != nil {
			return fmt.Errorf("certificate reloader: %w", err)
		}
	}
	return nil
}

func (m *MiddlewareConfig) BindFlags(fs *pflag.FlagSet) error {
	if fs == nil {
		fs = pflag.CommandLine
	}
	var err error
	if m.RecoveryConfig != nil && fs.Changed(FlagRecoveryEnabled) {
		if m.RecoveryConfig.Enabled, err = fs.GetBool(FlagRecoveryEnabled); err != nil {
			return fmt.Errorf("recovery: %w", err)
		}
	}
	if m.CertificateAuthConfig != nil {
		if err = m.CertificateAuthConfig.BindFlags(fs); err != nil {
			return fmt.Errorf("certificateAuth: %w", err)
		}
	}
	if m.NPAAuthConfig != nil && fs.Changed(FlagMiddlewareNPAAuthEnabled) {
		if m.NPAAuthConfig.Enabled, err = fs.GetBool(FlagMiddlewareNPAAuthEnabled); err != nil {
			return fmt.Errorf("npaAuth: %w", err)
		}
	}
	if m.UserAuthConfig != nil && fs.Changed(FlagMiddlewareUserAuthEnabled) {
		if m.UserAuthConfig.Enabled, err = fs.GetBool(FlagMiddlewareUserAuthEnabled); err != nil {
			return fmt.Errorf("userAuth: %w", err)
		}
	}
	if m.MetricsConfig != nil && fs.Changed(FlagMiddlewareMetricsEnabled) {
		if m.MetricsConfig.Enabled, err = fs.GetBool(FlagMiddlewareMetricsEnabled); err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
	}
	if m.LoggerConfig != nil && fs.Changed(FlagMiddlewareLoggerEnabled) {
		if m.LoggerConfig.Enabled, err = fs.GetBool(FlagMiddlewareLoggerEnabled); err != nil {
			return fmt.Errorf("logger: %w", err)
		}
	}
	if m.RequestIDConfig != nil && fs.Changed(FlagMiddlewareRequestIDEnabled) {
		if m.RequestIDConfig.Enabled, err = fs.GetBool(FlagMiddlewareRequestIDEnabled); err != nil {
			return fmt.Errorf("requestID: %w", err)
		}
	}
	if m.GZIPConfig != nil {
		if fs.Changed(FlagMiddlewareGZIPEnabled) {
			if m.GZIPConfig.Enabled, err = fs.GetBool(FlagMiddlewareGZIPEnabled); err != nil {
				return fmt.Errorf("gzip: %w", err)
			}
		}
		if fs.Changed(FlagMiddlewareGZIPCompression) {
			var gzipLevel int
			if gzipLevel, err = fs.GetInt(FlagMiddlewareGZIPCompression); err != nil {
				return fmt.Errorf("gzipCompression: %w", err)
			}
			m.GZIPConfig.CompressionMode = gzip.CompressionMode(gzipLevel)
		}
	}
	if m.TraceConfig != nil && fs.Changed(FlagMiddlewareTraceEnabled) {
		if m.TraceConfig.Enabled, err = fs.GetBool(FlagMiddlewareTraceEnabled); err != nil {
			return fmt.Errorf("trace: %w", err)
		}
	}
	return nil
}

func (c *Config) ApplyDefaults() error {
	if c.Mode == "" {
		c.Mode = DefaultMode
	}
	c.Middleware.ApplyDefaults()
	c.TLSConfig.ApplyDefaults()
	c.HTTPServer.ApplyDefaults()
	c.ServiceConfig.ApplyDefaults()
	// if reloader is enabled and no files are specified, and TLS is used, watch the cert file
	if c.ServiceConfig.Reloader.Enabled {
		if len(c.ServiceConfig.Reloader.Files) == 0 && c.TLSConfig.UseTLS() {
			c.ServiceConfig.Reloader.Files = []string{c.TLSConfig.Cert}
		}
	}
	return c.injectServerCNs()
}

func (c *Config) injectServerCNs() error {
	if !c.TLSConfig.UseTLS() {
		return nil
	}
	if c.Middleware.CertificateAuthConfig == nil || !c.Middleware.CertificateAuthConfig.Enabled {
		return nil
	}
	if c.Middleware.CertificateAuthConfig.SkipServerCNInjection {
		return nil
	}

	// Load the certificate from the webserver's TLS configuration
	keypair, err := tls.LoadX509KeyPair(c.TLSConfig.Cert, c.TLSConfig.Key)
	if err != nil {
		return fmt.Errorf("failed to load webserver certificate: %w", err)
	}

	// Parse the certificate from the keypair
	var certs []*x509.Certificate
	for _, certDER := range keypair.Certificate {
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}

	// Extract CNs and DNS names from the certificate
	cns := tlsutils.ExtractDNSNames(certs)
	if len(cns) == 0 {
		return nil
	}

	// Create a certificate entry with the extracted CNs and DNS names
	ownedCertificate := certificate.Certificate{
		Scopes: []scope.Scope{
			&basic.Scope{
				Actions:      []string{"*"},
				Environments: []string{"*"},
				Teams:        []string{"*"},
				Roles:        []string{"*"},
			},
		},
		CNs: cns,
	}

	// Append to the certificate authentication middleware configuration
	c.Middleware.CertificateAuthConfig.Certificates = append(
		c.Middleware.CertificateAuthConfig.Certificates,
		ownedCertificate,
	)
	return nil
}

func (s *ServiceConfig) ApplyDefaults() {
	if s.PProfConfig == nil {
		s.PProfConfig = &PProfConfig{Enabled: DefaultPProfEnabled}
	}
	if s.MetricConfig == nil {
		s.MetricConfig = &MetricConfig{Path: DefaultMetricsPath, Enabled: DefaultMetricsEnabled}
	}
	if s.Healthcheck == nil {
		s.Healthcheck = &HealthCheckConfig{Enabled: DefaultHealthcheckEnabled}
	}
	if s.Reloader == nil {
		s.Reloader = &ReloaderConfig{Enabled: DefaultReloaderEnabled}
	}
	if s.ProxyConfig == nil {
		s.ProxyConfig = &ProxyConfig{Enabled: DefaultProxyEnabled}
	}
}

func (m *MiddlewareConfig) ApplyDefaults() {
	if m.RecoveryConfig == nil {
		m.RecoveryConfig = &RecoveryConfig{Enabled: DefaultRecoveryEnabled}
	}
	if m.MetricsConfig == nil {
		m.MetricsConfig = &metrics.Config{Enabled: DefaultMetricsEnabled, SkipPaths: metrics.DefaultSkipPaths}
	}
	if m.LoggerConfig == nil {
		m.LoggerConfig = &logger.Config{Enabled: DefaultLoggerEnabled, SkipPaths: logger.DefaultSkipPaths}
	}
	if m.RequestIDConfig == nil {
		m.RequestIDConfig = &requestid.Config{Enabled: DefaultRequestIDEnabled}
	}
	if m.GZIPConfig == nil {
		m.GZIPConfig = &gzip.Config{Enabled: DefaultGZIPEnabled, CompressionMode: gzip.DefaultCompression}
	}
	if m.UserAuthConfig == nil {
		m.UserAuthConfig = &user.Config{Enabled: DefaultUserAuthEnabled}
	}
	if m.NPAAuthConfig == nil {
		m.NPAAuthConfig = &npa.Config{Enabled: DefaultNPAAuthEnabled}
	}
	if m.CertificateAuthConfig == nil {
		m.CertificateAuthConfig = &certificate.Config{Enabled: DefaultCertificateAuthEnabled}
	} else {
		m.CertificateAuthConfig.ApplyDefaults()
	}
	if m.OauthConfig == nil {
		m.OauthConfig = &oauth.Config{Enabled: DefaultOAuthEnabled}
	}
	if m.TraceConfig == nil {
		m.TraceConfig = &TraceConfig{Enabled: DefaultTraceEnabled}
	}
}

func (c *Config) Validate() error {
	if c.TLSConfig.UseTLS() {
		if err := c.TLSConfig.Validate(); err != nil {
			return err
		}
	}
	if err := c.HTTPServer.Validate(); err != nil {
		return err
	}
	if c.Middleware.CertificateAuthConfig != nil && c.Middleware.CertificateAuthConfig.Enabled {
		if err := c.Middleware.CertificateAuthConfig.Validate(); err != nil {
			return err
		}
		if !c.TLSConfig.UseTLS() {
			return fmt.Errorf("TLSConfig is required when MTLSConfig.Enabled")
		}
		clientAuthType, err := tlsutils.ParseClientAuthType(c.TLSConfig.ClientAuthType)
		if err != nil {
			return err
		}
		if clientAuthType == tls.NoClientCert || clientAuthType == tls.VerifyClientCertIfGiven || clientAuthType == tls.RequestClientCert {
			return fmt.Errorf("ClientAuthType must be RequireAndVerifyClientCert or RequireAnyClientCert when MTLSConfig.Enabled")
		}
	}
	return nil
}
