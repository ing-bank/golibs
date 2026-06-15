// Package ginresponse is EXPERIMENTAL: These functions are still in flux. Its signature, behavior, or semantics may
// change without notice in upcoming releases.
package ginresponse

import (
	"io"
	"net/http"

	"github.com/ing-bank/golibs/pkg/slices"
)

type Response struct {
	Body       any // Mutually exclusive with Stream and Err
	StatusCode int
	Headers    map[string]string
	Err        error     // Body is ignored when Err is set
	Stream     io.Reader // Body is ignored when Stream is set

	//ChunckedStream     func(writer io.Writer) bool // Step function for streaming responses
}

func New(body any) *Response {
	return &Response{
		StatusCode: http.StatusOK,
		Body:       body,
	}
}

func NewWithError(err error) *Response {
	return &Response{
		Err: err,
	}
}

func NewWithStream(body io.Reader) *Response {
	return &Response{
		StatusCode: http.StatusOK,
		Stream:     body,
	}
}

func (r *Response) WithStatus(code int) *Response {
	r.StatusCode = code
	return r
}

func (r *Response) WithHeaders(headers map[string]string) *Response {
	r.Headers = slices.MergeMap(r.Headers, headers)
	return r
}

func (r *Response) WithHeader(key, value string) *Response {
	if r.Headers == nil {
		r.Headers = map[string]string{}
	}
	r.Headers[key] = value
	return r
}

// WithError overrides any payload. Also, the status code must either be unset or set to a proper
// http error status code.
func (r *Response) WithError(err error) *Response {
	r.Err = err
	return r
}
