package metrics

import (
	"context"
	"fmt"
	gohttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/ginserver/manager"
	"github.com/ing-bank/golibs/pkg/healthcheck"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/httpserver"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type App struct{}

func (a *App) Get200(c *gin.Context) {
	c.String(gohttp.StatusOK, "Hello World!")
}

func (a *App) Register(rg gin.IRouter) {
	rg.GET("/healthz", a.Get200)
}

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load server configuration. Usually this is a subsection of your own config. In this example
	// we set up a manager (a Webserver with a Sidecar server). The sidecar server has a default
	// telnet health check that checks the main webserver health at a certain interval.
	cfg := manager.Config{
		Config: ginserver.Config{
			HTTPServer: httpserver.Config{
				Port: 8084,
			},
			ServiceConfig: ginserver.ServiceConfig{
				Healthcheck: &ginserver.HealthCheckConfig{Enabled: false}, // We override the health checks with our own dummy endpoint, see above TODO: use health check package properly
			},
		},
		SidecarConfig: manager.SidecarConfig{Port: 8085},
		SidecarServiceConfig: ginserver.ServiceConfig{
			Healthcheck: &ginserver.HealthCheckConfig{
				Enabled: true, // Default
				Config: healthcheck.Config{
					Interval: metav1.Duration{Duration: 100 * time.Millisecond},
				},
			},
		},
	}

	// This is an example app. It has a Register method to populate the server
	app := &App{}

	router, err := manager.NewManager(&cfg,
		manager.WithWebserverOptions(ginserver.WithRoutes(app)),
	)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	go func() {
		time.Sleep(2 * time.Second) // Give health check some time to run job and populate cache
		lookup := map[string]any{}
		resp := http.DefaultClient.Get(ctx, "http://localhost:8085/healthz")
		if err := resp.Parse(&lookup).Error(); err != nil {
			log.Errorf("Failed to get healthz: %v", err)
			return
		}
		fmt.Println(lookup["status"])
		cancel() // Cancel the context to stop servers after successful check
	}()

	if err := router.Run(ctx); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	// Output:
	// OK
}
