package healthcheck

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/ginresponse"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/task/job"
)

type EndpointOption struct {
	Root    Endpoint
	Readyz  Endpoint
	Healthz Endpoint
	Status  Endpoint
}

func (h *HealthCheck) Register(rg gin.IRouter) {
	h.RouteRegister(rg)
}

func (h *HealthCheck) HandlerFuncReadyz(c *gin.Context) *ginresponse.Response {
	jobs := h.JobsForEndpoint(ReadyEndpoint)
	code, result := h.handle(c.Request.Context(), jobs)
	return ginresponse.New(result).WithStatus(code)
}

func (h *HealthCheck) HandlerFuncHealthz(c *gin.Context) *ginresponse.Response {
	jobs := h.JobsForEndpoint(HealthEndpoint)
	code, result := h.handle(c.Request.Context(), jobs)
	return ginresponse.New(result).WithStatus(code)
}

func (h *HealthCheck) HandlerFuncStatus(c *gin.Context) *ginresponse.Response {
	jobs := h.AllChecks()
	code, result := h.handle(c.Request.Context(), jobs)
	return ginresponse.New(result).WithStatus(code)
}

func (h *HealthCheck) handle(ctx context.Context, jobs []job.Job) (int, any) {
	result := h.run(ctx, jobs, h.Component)
	return AvailabilityFromString(result.Status), result
}

func (h *HealthCheck) RouteRegister(rg gin.IRouter, prefixOpt ...EndpointOption) {

	wrapper, err := ginresponse.NewWrapper(ginresponse.SkipResponseLog(true))
	if err != nil {
		panic(err)
	}

	prefix := opt.Opt(EndpointOption{
		Root:    RootEndpoint,
		Readyz:  ReadyEndpoint,
		Healthz: HealthEndpoint,
		Status:  StatusEndpoint,
	}, prefixOpt)
	prefixRouter := rg.Group(prefix.Root.String())
	{
		prefixRouter.GET(prefix.Status.String(), wrapper.Wrap(h.HandlerFuncStatus))
		if h.HasEndpoint(HealthEndpoint) {
			prefixRouter.GET(prefix.Healthz.String(), wrapper.Wrap(h.HandlerFuncHealthz))
		}
		if h.HasEndpoint(ReadyEndpoint) {
			prefixRouter.GET(prefix.Readyz.String(), wrapper.Wrap(h.HandlerFuncReadyz))
		}
	}
}
