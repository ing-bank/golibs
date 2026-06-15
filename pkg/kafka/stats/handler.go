package stats

import (
	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginresponse"
	"github.com/ing-bank/golibs/pkg/opt"
)

const (
	RootEndpoint   Endpoint = "/kafka"
	StatsEndpoint  Endpoint = "/stats"
	StatusEndpoint Endpoint = "/status"
)

// Endpoint is a string type
type Endpoint string

func (e Endpoint) String() string {
	return string(e)
}

type Handler interface {
	HandleStats(c *gin.Context) *ginresponse.Response
	HandleStatus(c *gin.Context) *ginresponse.Response
}

type EndpointOption struct {
	Root   Endpoint
	Stats  Endpoint
	Status Endpoint
}

func RouteRegister(rg gin.IRouter, h Handler, prefixOpt ...EndpointOption) {
	wrapper, err := ginresponse.NewWrapper(ginresponse.SkipResponseLog(true))
	if err != nil {
		panic(err)
	}

	prefix := opt.Opt(EndpointOption{
		Root:   RootEndpoint,
		Stats:  StatsEndpoint,
		Status: StatusEndpoint,
	}, prefixOpt)
	prefixRouter := rg.Group(prefix.Root.String())
	{
		prefixRouter.GET(prefix.Stats.String(), wrapper.Wrap(h.HandleStats))
		prefixRouter.GET(prefix.Status.String(), wrapper.Wrap(h.HandleStatus))
	}
}
