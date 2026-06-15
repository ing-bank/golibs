package main

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginserver"
	log "github.com/sirupsen/logrus"
)

type App struct {
	Text string
}

func (a *App) Get200(c *gin.Context) {
	c.String(http.StatusOK, a.Text)
}

func (a *App) Register(rg gin.IRouter) {
	rg.GET("/v1/example", a.Get200)
}

func main() {
	ctx := context.Background()

	// Load server configuration. Usually this is a subsection of your own config. We just load
	// the defaults here.
	ginConfig := ginserver.DefaultConfig()

	// This is an example app. It has a Register method to populate the server
	app := &App{Text: "Hello, World!"}

	router, err := ginserver.NewForConfig(ginConfig, ginserver.WithRoutes(app))
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	if err := router.Run(ctx); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
