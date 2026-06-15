package _1_simple

import (
	"context"
	"fmt"
	gohttp "net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/access"
	"github.com/ing-bank/golibs/pkg/access/scope"
	"github.com/ing-bank/golibs/pkg/access/scope/basic"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/httpserver"
	"github.com/ing-bank/golibs/pkg/middleware/authorization/user"
)

type App struct{}

func LogAccess(c *gin.Context) {
	account := access.GetTrustFromGinContext(c)

	// Example output:
	// GET / -> {User foo [{[*] [dev tst] [team1] [user]} {[GET] [acc prd] [team1] [user]}]}
	fmt.Printf("%s %s -> %s", c.Request.Method, c.Request.RequestURI, account)

	c.String(200, "Hello, World!")
}

// Register allows our app to work with the default GinServer provided by GoLibs
func (a *App) Register(rg gin.IRouter) {
	rg.Any("/", LogAccess) // Any method on / will call LogAccess
}

// CustomUserScopeParser implements user.ScopeParser for basic.Scope
type CustomUserScopeParser struct{}

func (p *CustomUserScopeParser) ParseUserHeader(c *gin.Context) []scope.Scope {
	// Extract team from header (comma-separated values)
	teamHeader := c.GetHeader("team")
	teams := []string{"team1"} // Default team
	if teamHeader != "" {
		teams = strings.Split(teamHeader, ",")
	}

	// Return multiple scopes based on your authorization policy
	return []scope.Scope{
		&basic.Scope{ // Users can do anything in dev and tst
			Actions:      []string{scope.Wildcard},
			Environments: []string{"dev", "tst"},
			Teams:        teams,
			Roles:        []string{"user"},
		},
		&basic.Scope{ // Read only access in acc and prd
			Actions:      []string{gohttp.MethodGet},
			Environments: []string{"acc", "prd"},
			Teams:        teams,
			Roles:        []string{"user"},
		},
	}
}

func Example() { // Main
	const (
		UsernameHeader = "user"
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	app := &App{} // App exposes Register method so we can use it 'WithRoutes'

	// We create a new server. We configure it inline, but usually you would
	// load this from a config file.
	router, _ := ginserver.NewForConfig(&ginserver.Config{
		HTTPServer: httpserver.Config{
			Port: 8080,
		},
		Middleware: ginserver.MiddlewareConfig{UserAuthConfig: &user.Config{
			Enabled:        true,
			UsernameHeader: UsernameHeader,
			ScopeParser:    &CustomUserScopeParser{},
		}},
	}, ginserver.WithRoutes(app)) // <- Register our application

	// Perform a client request with headers to simulate a logged-in user trying an action
	go func() {
		// Normally these headers are set by the Arch. The Arch logs in AD / Entra users and sets headers.
		headers := map[string]string{
			UsernameHeader: "foo",
			"team":         "team1",
		}
		_ = http.DefaultClient.Get(ctx, "http://localhost:8080/", http.WithHeaders(headers))
	}()

	if err := router.Run(ctx); err != nil {
		panic(err)
	}

	// Output:
	// GET / -> {User foo [{[*] [dev tst] [team1] [user]} {[GET] [acc prd] [team1] [user]}]}
}
