package simple

import (
	"context"
	"fmt"
	gohttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/npa"
)

type App struct{}

func LogAccess(c *gin.Context) {
	account := access.GetTrustFromGinContext(c)
	fmt.Printf("%s %s -> %s", c.Request.Method, c.Request.RequestURI, account)
	c.String(200, "Hello, World!")
}

// Register allows our app to work with the default GinServer provided by GoLibs
func (a *App) Register(rg gin.IRouter) {
	rg.Any("/", LogAccess)
}

func Example() { // Main
	const (
		NPAHeader = "npa"
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app := &App{} // App exposes Register method so we can use it 'WithRoutes'

	// We create a new server. We configure it inline, but usually you would
	// load this from a config file.
	router, _ := ginserver.NewForConfig(&ginserver.Config{
		HTTPServer: httpserver.Config{
			Port: 8081,
		},
		Middleware: ginserver.MiddlewareConfig{NPAAuthConfig: &npa.Config{
			Enabled: true,
			Header:  NPAHeader,
			AllowedNPAs: []npa.AllowedNPA{
				{
					Name: "FOO_NPA",
					Scopes: []scope.Scope{
						basic.Scope{
							Actions:      []string{gohttp.MethodGet},
							Environments: []string{scope.Wildcard},
							Teams:        []string{"team1"},
							Roles:        []string{"user"},
						},
					},
				},
				{
					Name: "BAR_NPA",
					Scopes: []scope.Scope{
						basic.Scope{
							Actions:      []string{scope.Wildcard},
							Environments: []string{scope.Wildcard},
							Teams:        []string{"team2"},
							Roles:        []string{"admin"},
						},
					},
				},
			},
		}},
	}, ginserver.WithRoutes(app)) // <- Register our application

	// Perform a client request with headers to simulate an NPA trying an action
	go func() {
		// Normally these headers are set by an authenticating gateway, but we set them manually here for testing
		headers := map[string]string{
			NPAHeader: "FOO_NPA",
		}
		_ = http.DefaultClient.Get(ctx, "http://localhost:8081/", http.WithHeaders(headers))
	}()

	if err := router.Run(ctx); err != nil {
		panic(err)
	}

	// Output:
	// GET / -> {NPA FOO_NPA [{[GET] [*] [team1] [user]}]}
}
