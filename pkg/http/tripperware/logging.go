package tripperware

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ing-bank/golibs/pkg/http/response"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/slices"
	log "github.com/sirupsen/logrus"
)

var defaultSkipPaths = []string{"/metrics", "/healthz", "/readyz", "/swagger"}

type LoggingOptions struct {
	DisabledRequestLogger   bool     `yaml:"disabledRequestLogger" json:"disabledRequestLogger,omitempty"`     // Default false
	DisabledResponseLogger  bool     `yaml:"disabledResponseLogger" json:"disabledResponseLogger,omitempty"`   // Default false
	LogFailedRequests       bool     `yaml:"logFailedRequests" json:"logFailedRequests,omitempty"`             // Default false
	RequestCutoffThreshold  int      `yaml:"requestCutoffThreshold" json:"requestCutoffThreshold,omitempty"`   // Default 10240
	ResponseCutoffThreshold int      `yaml:"responseCutoffThreshold" json:"responseCutoffThreshold,omitempty"` // Default 1024
	SkipPaths               []string `yaml:"skipPaths" json:"skipPaths,omitempty"`                             // Default defaultSkipPaths
}

func (opts *LoggingOptions) SetDefaults() {
	if opts.RequestCutoffThreshold == 0 {
		opts.RequestCutoffThreshold = 10240
	}
	if opts.ResponseCutoffThreshold == 0 {
		opts.ResponseCutoffThreshold = 1024
	}
	if opts.SkipPaths == nil {
		opts.SkipPaths = defaultSkipPaths
	}
}

func Logging(options ...LoggingOptions) Tripperware {
	opts := opt.Opt(LoggingOptions{}, options) // Keep same function signature as before
	opts.SetDefaults()

	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, request *http.Request) *response.Data {

			// TODO: use log.Fields with request, response. Sadly this is not (easily) human readable via stdout
			// TODO: Perhaps we should use a flag for JSON logging and just Tracing only

			method := request.Method
			url := request.URL

			if slices.Contains(opts.SkipPaths, url.Path) {
				// Do not print health calls
				return next(ctx, request)
			}

			// Log request
			if !opts.DisabledRequestLogger {
				log.WithContext(ctx).Infof("[HttpClient] Executing %s %s with request: %s", method, url, readRequest(request, opts.RequestCutoffThreshold))
			}

			// Call next tripperware
			resp := next(ctx, request)
			if err := resp.Error(); err != nil {
				if opts.LogFailedRequests {
					log.WithContext(ctx).Errorf("[HttpClient] Error executing %s %s with request: %s, status: %d, error: %v", method, url, readRequest(request, opts.RequestCutoffThreshold), resp.Status, err)
				}

				if ver, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
					var summary []string
					for _, cert := range ver.UnverifiedCertificates {
						summary = append(summary, fmt.Sprintf("Subject: '%s' Expires: '%s' Issuer: '%s'", cert.Subject.String(), cert.NotAfter, cert.Issuer.String()))
					}
					log.WithContext(ctx).Errorf("[HttpClient] Certificate Verification Error %v: [%v]", ver, strings.Join(summary, ", "))
				}

				// although there was an error, we may have a response to log
				if !opts.DisabledResponseLogger {
					log.WithContext(ctx).Errorf("[HttpClient] %s %s had status: %d, error: %v, response: %s", method, url, resp.Status, err, readResponse(resp, opts.ResponseCutoffThreshold))
				} else {
					log.WithContext(ctx).Errorf("[HttpClient] %s %s had status %d, error: %v", method, url, resp.Status, err)
				}

				// Return early on error
				return resp
			}

			// Log response
			if !opts.DisabledResponseLogger {
				log.WithContext(ctx).Infof("[HttpClient] %s %s had status: %d, response: %s", method, url, resp.Status, readResponse(resp, opts.ResponseCutoffThreshold))
			} else {
				log.WithContext(ctx).Infof("[HttpClient] %s %s had status %d", method, url, resp.Status)
			}

			return resp
		}
	}
}

func readResponse(resp *response.Data, responseCutoffThreshold int) string {
	if resp == nil || resp.Raw == nil {
		return ""
	}
	if len(resp.Raw) > responseCutoffThreshold {
		return string(resp.Raw[:responseCutoffThreshold]) + "..."
	}
	return string(resp.Raw[:])
}

func readRequest(request *http.Request, requestCutoffThreshold int) string {
	if requestCutoffThreshold <= 0 {
		return "..."
	}
	if request == nil || request.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(request.Body)
	if err != nil {
		return fmt.Sprintf("failed to read request body: %v", err)
	}
	request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if len(bodyBytes) > requestCutoffThreshold {
		return string(bodyBytes[:requestCutoffThreshold]) + "..."
	}
	return string(bodyBytes)
}
