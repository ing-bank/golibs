package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	resp "github.com/ing-bank/golibs/pkg/ginresponse"
	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/httpserver"
	log "github.com/sirupsen/logrus"
)

type App struct{}

func (a *App) Handler(_ *gin.Context) *resp.Response {
	payload := gin.H{"message": "Hello, World!"}
	return resp.New(payload)
}

func (a *App) HandlerWithHeader(_ *gin.Context) *resp.Response {
	payload := gin.H{"message": "Hello, World!"}
	return resp.New(payload).WithHeader("foo", "bar")
}

func (a *App) HandlerWithStatus(_ *gin.Context) *resp.Response {
	payload := gin.H{"message": "Hello, World!"}
	return resp.New(payload).WithStatus(202)
}

func (a *App) HandlerWithMultipleOptions(_ *gin.Context) *resp.Response {
	payload := gin.H{"message": "Hello, World!"}
	return resp.New(payload).WithHeader("foo", "bar").WithStatus(202)
}

func (a *App) HandlerWithErr(_ *gin.Context) *resp.Response {
	err := http.ErrConflict // Is translated to 409 by errors package default error code translator
	return resp.NewWithError(err)
}

func (a *App) Register(rg gin.IRouter) {
	w, _ := resp.NewWrapper()

	rg.GET("/handler", w.Wrap(a.Handler))
	rg.GET("/handler-header", w.Wrap(a.HandlerWithHeader))
	rg.GET("/handler-status", w.Wrap(a.HandlerWithStatus))
	rg.GET("/handler-multi", w.Wrap(a.HandlerWithMultipleOptions))
	rg.GET("/handler-err", w.Wrap(a.HandlerWithErr))
}

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Load server configuration. Usually this is a subsection of your own config. We just load
	// the defaults here.
	cfg := ginserver.Config{
		HTTPServer: httpserver.Config{
			Host: "127.0.0.1",
			Port: 8086,
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

		// Server is ready, make the request
		response := http.DefaultClient.Get(ctx, "http://localhost:8086/handler")
		if response.IsOK() {
			fmt.Println(response.Status, string(response.Raw[:]))
		}
	}()

	if err := router.Run(ctx); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

	// Output:
	// 200 {"message":"Hello, World!"}
}
