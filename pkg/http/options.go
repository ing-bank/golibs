package http

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/http/tripperware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ClientOption allows options to be set on Client that will be used on every request
type ClientOption = config.Option[*Client]

// RequestOption allows options to be set per request
type RequestOption = config.Option[*http.Request]

// With applies the provided options to the client
func (c *Client) With(opts ...ClientOption) error {
	return config.ApplyOpts(c, opts...)
}

// WithTripperware sets the tripperware chain to be used by the client
func WithTripperware(chain tripperware.Tripperware) config.Opt[*Client] {
	return func(c *Client) error {
		c.Tripperware = chain
		return nil
	}
}

// WithTransport sets the transport to be used by the client
func WithTransport(transport *http.Transport) config.Opt[*Client] {
	return func(c *Client) error {
		c.Http.Transport = transport
		return nil
	}
}

// WithTraceTransport wraps the provided transport with OpenTelemetry tracing
func WithTraceTransport(transport *http.Transport) config.Opt[*Client] {
	return func(c *Client) error {
		c.Http.Transport = otelhttp.NewTransport(transport)
		return nil
	}
}

// WithNewTraceTransport creates new transport with the provided TransportOptions
// and wraps it with OpenTelemetry tracing
func WithNewTraceTransport(options ...TransportOption) config.Opt[*Client] {
	return func(c *Client) error {
		transport, err := NewTransport(options...)
		if err != nil {
			return err
		}
		c.Http.Transport = otelhttp.NewTransport(transport)
		return nil
	}
}

// WithNewTransport creates new transport with the provided TransportOptions
func WithNewTransport(options ...TransportOption) config.Opt[*Client] {
	return func(c *Client) error {
		transport, err := NewTransport(options...)
		if err != nil {
			return err
		}
		return WithTransport(transport)(c)
	}
}

// WithCheckRedirect specifies the policy for handling redirects
func WithCheckRedirect(redirect func(req *http.Request, via []*http.Request) error) config.Opt[*Client] {
	return func(c *Client) error {
		c.Http.CheckRedirect = redirect
		return nil
	}
}

// WithRequestOptions sets default RequestOptions for every request the client makes
func WithRequestOptions(opts ...RequestOption) config.Opt[*Client] {
	return func(c *Client) error {
		c.DefaultRequestOptions = opts
		return nil
	}
}

// --- Request DefaultRequestOptions below, can be set as defaults for a client via WithRequestOptions ---

// WithBasicAuth sets the username and password as basic authentication
func WithBasicAuth(username, password string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		request.SetBasicAuth(username, password)
		return nil
	}
}

// WithBasicAuthEnv looks up the user and password in environment variables and then sets them as basic auth
func WithBasicAuthEnv(envUser, envPass string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		username := os.Getenv(envUser)
		if username == "" {
			return fmt.Errorf("%w: '%s' is not set", ErrEnvironmentVariable, envUser)
		}
		username = strings.TrimSuffix(username, "\n")

		password := os.Getenv(envPass)
		if password == "" {
			return fmt.Errorf("%w: the environment variable '%s' is not set", ErrEnvironmentVariable, envPass)
		}
		password = strings.TrimSuffix(password, "\n")

		request.SetBasicAuth(username, password)
		return nil
	}
}

// WithBearerAuth sets the Bearer token for authorization
// If the token is empty, it tries to get the token from the in-cluster kubeconfig
// or from the default kubeconfig file in the local environment
func WithBearerAuth(token string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
			return nil
		}
		kubeconfig, err := ctrl.GetConfig()
		if err != nil {
			return fmt.Errorf("failed to get kubeconfig: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+kubeconfig.BearerToken)
		return nil
	}
}

// WithHeaders sets the HTTP headers for a request
func WithHeaders(headers map[string]string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		for header, val := range headers {
			request.Header.Set(header, val)
		}
		return nil
	}
}

// WithAddHeaders sets the HTTP headers for a request
func WithAddHeaders(headers map[string]string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		for header, val := range headers {
			request.Header.Add(header, val)
		}
		return nil
	}
}

// WithParams adds query parameters to the request URL
func WithParams(params map[string]string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		q := request.URL.Query()
		for param, val := range params {
			q.Add(param, val)
		}
		request.URL.RawQuery = q.Encode()
		return nil
	}
}

// WithRawQuery sets the raw query string for the request URL
func WithRawQuery(rawQuery string) config.Opt[*http.Request] {
	return func(request *http.Request) error {
		request.URL.RawQuery = rawQuery
		return nil
	}
}
