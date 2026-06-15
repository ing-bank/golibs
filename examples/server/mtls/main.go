package main

import (
	"context"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/ginserver/manager"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	"github.com/ing-bank/golibs/pkg/healthcheck/checks"
	"github.com/ing-bank/golibs/pkg/logging"
	"github.com/ing-bank/golibs/pkg/task/job"
	"github.com/ing-bank/golibs/pkg/trace"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const APIVersion = "/v1/example"

type App struct {
	Text       string
	middleware []gin.HandlerFunc
}

func (a *App) GetV1Example(status string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.String(http.StatusOK, status)
	}
}

func (a *App) Register(rg gin.IRouter) {
	v1capacity := ginserver.GroupWithMiddlewares(rg, APIVersion, a.middleware...)
	v1capacity.GET("/", a.GetV1Example(a.Text))
}

type Config struct {
	Logging logging.Config `json:"logging" yaml:"logging"`
	Server  manager.Config `json:"server" yaml:"server"`
	Tracing trace.Config   `json:"tracing" yaml:"tracing"`
	APIs    map[string]API `json:"apis" yaml:"apis"`
}

type API struct {
	Path       string                      `json:"path" yaml:"path"`
	Enabled    bool                        `json:"enabled" yaml:"enabled"`
	Middleware *ginserver.MiddlewareConfig `json:"middleware" yaml:"middleware"`
}

func (c *Config) BindFlags(fs *pflag.FlagSet) error {
	return config.ChainFlags(fs, &c.Server, &c.Logging)
}

func (c *Config) Validate() error {
	return config.ChainValidations(&c.Server, &c.Logging, &c.Tracing)
}

func NewApp(user API) (*App, error) {
	middleware, err := ginserver.NewMiddlewareForConfig(user.Middleware)
	if err != nil {
		log.Fatalf("Failed to create middleware for API %s: %v", APIVersion, err)
	}
	return &App{
		Text:       "Hello, World!",
		middleware: middleware,
	}, nil
}

func main() {

	ginserver.RegisterFlags(pflag.CommandLine)
	logging.RegisterFlags(pflag.CommandLine)
	trace.RegisterFlags(pflag.CommandLine)

	pflag.Parse()

	cfgPath, ok := os.LookupEnv("APP_CONFIG")
	if !ok {
		cfgPath = "examples/server/mtls/example.yaml"
	}

	cfg, err := config.LoadType[Config](cfgPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Loaded configuration: %+v", cfg)

	if err := cfg.BindFlags(pflag.CommandLine); err != nil {
		log.Fatalf("Failed to bind logging flags: %v", err)
	}

	logging.SetLogFormatter(&cfg.Logging)

	user, err := NewApp(cfg.APIs[APIVersion])
	if err != nil {
		log.Fatalf("Failed to create user API: %v", err)
	}

	m, err := manager.NewManager(&cfg.Server,
		manager.WithWebserverOptions(ginserver.WithRoutes(user)),
		manager.WithSidecarOptions(
			ginserver.WithHealthChecks(
				healthcheck.WithAddChecks(
					healthcheck.JobConfig{
						Config: job.Config{
							Name: "custom-healthcheck",
						},
						CustomHandler: checks.HandlerFunc(func(ctx context.Context) error {
							return nil
						}),
						Endpoints: []healthcheck.Endpoint{
							healthcheck.HealthEndpoint,
							healthcheck.ReadyEndpoint,
						},
					},
				),
			),
		),
		manager.WithProxycarOptions(nil),
	)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	ctx := context.Background()
	for err := range m.RunBackground(ctx) {
		log.WithContext(ctx).Errorf("Server error: %v", err)
	}

}
