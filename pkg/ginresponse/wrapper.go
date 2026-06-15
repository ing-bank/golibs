package ginresponse

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var DefaultWrapper, _ = NewWrapper()

type Endpoint func(c *gin.Context) *Response

type Wrapper struct {
	ErrorToBody     func(err error) any
	ErrorToStatus   func(err error) int
	SkipResponseLog bool
	SkipPaths       []string
}

func NewWrapper(options ...Option) (*Wrapper, error) {
	w := &Wrapper{
		ErrorToBody: func(err error) any {
			return gin.H{"error": err.Error()}
		},
		ErrorToStatus:   errors.ErrAsStatusCode,
		SkipResponseLog: false,
	}

	for _, option := range options {
		if err := option(w); err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (w *Wrapper) Wrap(target Endpoint) func(*gin.Context) {
	return func(c *gin.Context) {
		if c.Request == nil || c.Request.URL == nil {
			return
		}

		var ctx context.Context = c
		if c.Request.Context() != nil {
			ctx = c.Request.Context()
		}

		if !w.SkipResponseLog {
			log.WithContext(ctx).Infof("[Server] %s %s from %s", c.Request.Method, c.Request.URL, c.ClientIP())
		}

		resp := target(c)
		// Handle nil response as NoContent
		if resp == nil {
			c.Status(http.StatusNoContent)
			if !w.SkipResponseLog {
				log.WithContext(ctx).Infof("[Server] replied to %s %s with no content", c.Request.Method, c.Request.URL)
			}
			return
		}
		// Set headers
		for header, value := range resp.Headers {
			c.Header(header, value)
		}

		// Error overrides
		if resp.Err != nil {
			resp.Body = w.ErrorToBody(resp.Err)
			if resp.StatusCode == 0 { // Set StatusCode for error if not set explicitly
				resp.StatusCode = w.ErrorToStatus(resp.Err)
			}
		}

		switch r := resp.Body.(type) {
		case error:
			if r != nil {
				c.String(resp.StatusCode, r.Error())
			} else {
				c.String(resp.StatusCode, "")
			}
			if !w.SkipResponseLog {
				log.WithContext(ctx).WithFields(log.Fields{
					"status": resp.StatusCode,
					"error":  r,
				}).Infof("[Server] replied to %s %s with an error", c.Request.Method, c.Request.URL)
			}
		case string:
			c.String(resp.StatusCode, r)
			if !w.SkipResponseLog {
				log.WithContext(ctx).WithFields(log.Fields{
					"status": resp.StatusCode,
					"body":   r,
				}).Infof("[Server] replied to %s %s", c.Request.Method, c.Request.URL)
			}
		case []byte:
			c.Data(resp.StatusCode, "application/octet-stream", r)
			if !w.SkipResponseLog {
				log.WithContext(ctx).WithFields(log.Fields{
					"status": resp.StatusCode,
					"size":   len(r),
				}).Infof("[Server] replied to %s %s with a byte response", c.Request.Method, c.Request.URL)
			}
		case io.Reader:
			_, err := io.Copy(c.Writer, r)
			if err != nil {
				if !w.SkipResponseLog {
					log.WithContext(ctx).WithFields(log.Fields{
						"status": resp.StatusCode,
						"error":  err,
					}).Errorf("[Server] failed to write response for %s %s", c.Request.Method, c.Request.URL)
				}
				_ = c.AbortWithError(http.StatusInternalServerError, err)
			} else if !w.SkipResponseLog {
				log.WithContext(ctx).WithFields(log.Fields{
					"status": resp.StatusCode,
				}).Infof("[Server] replied to %s %s with a stream response", c.Request.Method, c.Request.URL)
			}
		default:
			if !w.SkipResponseLog {
				log.WithContext(ctx).WithFields(log.Fields{
					"status": resp.StatusCode,
					"body":   resp.Body,
				}).Infof("[Server] replied to %s %s", c.Request.Method, c.Request.URL)
			}
			c.JSON(resp.StatusCode, resp.Body)
		}
	}
}

// Wrap is for legacy functions that return (int, any). It uses the
// DefaultWrapper.
//
// Deprecated: Use new Endpoint type with a Wrapper instance instead.
func Wrap[T any](target func(ctx *gin.Context) (int, T)) func(*gin.Context) {
	return DefaultWrapper.Wrap(func(c *gin.Context) *Response {
		status, body := target(c)
		return New(body).WithStatus(status)
	})
}
