package metrics

import (
	"context"
	"fmt"
	gohttp "net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/httpserver"
	log "github.com/sirupsen/logrus"
)

type App struct{}

func (a *App) Get200(c *gin.Context) {
	c.String(gohttp.StatusOK, "Hello World!")
}

func (a *App) Register(rg gin.IRouter) {
	rg.GET("/v1/example", a.Get200)
}

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load server configuration. Usually this is a subsection of your own config. We just load
	// the defaults here.
	cfg := ginserver.Config{
		HTTPServer: httpserver.Config{
			Host: "127.0.0.1",
			Port: 8083,
		},
		ServiceConfig: ginserver.ServiceConfig{
			MetricConfig: &ginserver.MetricConfig{
				Enabled: true,       // Default
				Path:    "/metrics", // Default
			},
		},
	}

	// This is an example app. It has a Register method to populate the server
	app := &App{}

	router, err := ginserver.NewForConfig(&cfg, ginserver.WithRoutes(app))
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	go func() {
		// Give the server some time to start
		time.Sleep(5 * time.Second)

		// Make a request to generate metrics
		_ = http.DefaultClient.Get(ctx, "http://localhost:8083/v1/example")

		// Server is ready, make the request
		resp := http.DefaultClient.Get(ctx, "http://localhost:8083/metrics")
		if resp == nil {
			log.Fatalf("No response received from metrics endpoint")
		}
		if !resp.IsOK() {
			log.Fatalf("Failed to get metrics: %s", resp.Error())
		}
		metrics := strings.SplitSeq(string(resp.Raw), "\n")
		for line := range metrics {
			if strings.Contains(line, "golibs_middleware_server_http_request_count_total{method=\"GET\",path=\"/v1/example\",status=\"200\"}") {
				fmt.Println(line)
			}
		}
	}()

	if err := router.Run(ctx); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	// Output:
	// golibs_middleware_server_http_request_count_total{method="GET",path="/v1/example",status="200"} 1
}
