// Package errors provides HTTP status code error types, conversion utilities, and retry logic
// for transport-level and API errors.
//
// HTTP Status Errors:
// The package defines error variables for common HTTP status codes (4xx and 5xx range),
// such as ErrNotFound (404), ErrConflict (409), ErrInternalServerError (500), etc.
// All HTTP errors use ErrHTTPStatus as a sentinel for error type checking.
//
// Error Conversion:
// Two main conversion functions bridge HTTP status codes and errors:
//
//   - StatusCodeAsErr: Converts an HTTP status code to its corresponding error type.
//   - ErrAsStatusCode: Converts an error to its HTTP status code (defaults to 500 for unknown errors).
//
// These functions also handle Kubernetes API errors automatically.
//
// Retry Logic:
// The package provides utilities to determine if an error is worth retrying:
//
//   - IsRetryableError: Returns true for temporary HTTP errors (timeouts, 5xx) and
//     transport-level errors (network disconnects, DNS issues, etc.).
//   - IsTransportRetryable: Checks if a transport-level error is transient
//     (timeout, syscall errors, mid-stream disconnects, etc.).
//   - AlwaysRetry: A simple retry function for always retrying.
//   - RetriableErrorFn: A pre-defined retry function using IsRetryableError.
//
// Helper Functions:
//
//   - IsNotFound: Checks if an error is a 404 Not Found.
//   - IsAlreadyExists: Checks if an error is a 409 Conflict or Kubernetes AlreadyExists.
//   - RunOnError: Returns true if an error is present.
//
// Example usage:
//
//	if errors.Is(err, errors.ErrNotFound) {
//		// Handle 404
//	}
package errors

import (
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ErrHTTPStatus is a sentinel error for HTTP status code related errors
var ErrHTTPStatus = goerrors.New("")

// ErrBadRequest is an error for HTTP 400 Bad Request
var ErrBadRequest = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusBadRequest)))

// ErrUnauthorized is an error for HTTP 401 Unauthorized
var ErrUnauthorized = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusUnauthorized)))

// ErrPaymentRequired is an error for HTTP 402 Payment Required
var ErrPaymentRequired = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusPaymentRequired)))

// ErrForbidden is an error for HTTP 403 Forbidden
var ErrForbidden = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusForbidden)))

// ErrNotFound is an error for HTTP 404 Not Found
var ErrNotFound = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusNotFound)))

// ErrMethodNotAllowed is an error for HTTP 405 Method Not Allowed
var ErrMethodNotAllowed = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusMethodNotAllowed)))

// ErrNotAcceptable is an error for HTTP 406 Not Acceptable
var ErrNotAcceptable = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusNotAcceptable)))

// ErrProxyAuthRequired is an error for HTTP 407 Proxy Authentication Required
var ErrProxyAuthRequired = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusProxyAuthRequired)))

// ErrRequestTimeout is an error for HTTP 408 Request Timeout
var ErrRequestTimeout = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusRequestTimeout)))

// ErrConflict is an error for HTTP 409 Conflict
var ErrConflict = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusConflict)))

// ErrGone is an error for HTTP 410 Gone
var ErrGone = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusGone)))

// ErrLocked is an error for HTTP 423 Locked
var ErrLocked = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusLocked)))

// ErrTooEarly is an error for HTTP 425 Too Early
var ErrTooEarly = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusTooEarly)))

// ErrTooManyRequests is an error for HTTP 429 Too Many Requests
var ErrTooManyRequests = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusTooManyRequests)))

// ErrInternalServerError is an error for HTTP 500 Internal Server Error
var ErrInternalServerError = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusInternalServerError)))

// ErrNotImplemented is an error for HTTP 501 Not Implemented
var ErrNotImplemented = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusNotImplemented)))

// ErrBadGateway is an error for HTTP 502 Bad Gateway
var ErrBadGateway = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusBadGateway)))

// ErrServiceUnavailable is an error for HTTP 503 Service Unavailable
var ErrServiceUnavailable = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusServiceUnavailable)))

// ErrGatewayTimeout is an error for HTTP 504 Gateway Timeout
var ErrGatewayTimeout = fmt.Errorf("%w%s", ErrHTTPStatus, strings.ToLower(http.StatusText(http.StatusGatewayTimeout)))

// Non HTTP errors

var ErrBuildRequest = errors.New("failed to build request")
var ErrEnvironmentVariable = errors.New("environment variable not set")
var ErrInvalidResponse = errors.New("invalid response")
var ErrFailedRequestOption = errors.New("failed to set request option")

var (
	lookupStatusAsErr map[int]error
	lookupErrAsStatus map[error]int
)

func init() {
	lookupStatusAsErr = map[int]error{
		http.StatusBadRequest:          ErrBadRequest,
		http.StatusUnauthorized:        ErrUnauthorized,
		http.StatusPaymentRequired:     ErrPaymentRequired,
		http.StatusForbidden:           ErrForbidden,
		http.StatusNotFound:            ErrNotFound,
		http.StatusMethodNotAllowed:    ErrMethodNotAllowed,
		http.StatusNotAcceptable:       ErrNotAcceptable,
		http.StatusProxyAuthRequired:   ErrProxyAuthRequired,
		http.StatusRequestTimeout:      ErrRequestTimeout,
		http.StatusConflict:            ErrConflict,
		http.StatusGone:                ErrGone,
		http.StatusTooEarly:            ErrTooEarly,
		http.StatusTooManyRequests:     ErrTooManyRequests,
		http.StatusInternalServerError: ErrInternalServerError,
		http.StatusNotImplemented:      ErrNotImplemented,
		http.StatusBadGateway:          ErrBadGateway,
		http.StatusLocked:              ErrLocked,
		http.StatusServiceUnavailable:  ErrServiceUnavailable,
		http.StatusGatewayTimeout:      ErrGatewayTimeout,
	}

	lookupErrAsStatus = map[error]int{}
	for status, err := range lookupStatusAsErr {
		lookupErrAsStatus[err] = status
	}
}

// IsNotFound returns true if the error indicates that the resource was not found.
func IsNotFound(err error) bool {
	if goerrors.Is(err, ErrNotFound) {
		return true
	}
	return apierrors.IsNotFound(err)
}

// IsAlreadyExists returns true if the error indicates that the resource already exists.
func IsAlreadyExists(err error) bool {
	if goerrors.Is(err, ErrConflict) {
		return true
	}
	return apierrors.IsAlreadyExists(err)
}

func StatusCodeAsErr(status int) error {
	if status > 0 && status < 400 {
		return nil
	}

	if err, ok := lookupStatusAsErr[status]; ok {
		return err
	}

	return ErrHTTPStatus
}

func ErrAsStatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}

	// Parse kubernetes errors to "our" http errors
	if apierr, ok := err.(*apierrors.StatusError); ok {
		err = StatusCodeAsErr(int(apierr.Status().Code))
	}

	for target, status := range lookupErrAsStatus {
		if errors.Is(err, target) {
			return status
		}
	}

	return http.StatusInternalServerError // Unknown error
}

// IsRetryableError returns true for the allowed list of API and transport errors
func IsRetryableError(err error) bool {
	// Parse Kubernetes errors to "our" HTTP errors
	if apierr, ok := err.(*apierrors.StatusError); ok {
		err = StatusCodeAsErr(int(apierr.Status().Code))
	}

	// Retry on known HTTP status errors
	if errors.Is(err, ErrHTTPStatus) {
		for _, target := range []error{
			ErrRequestTimeout,
			ErrTooEarly,
			ErrTooManyRequests,
			ErrInternalServerError,
			ErrBadGateway,
			ErrServiceUnavailable,
			ErrGatewayTimeout,
		} {
			if errors.Is(err, target) {
				return true
			}
		}
	}

	// Retry on transport-level errors
	// These include timeouts, temporary network failures, DNS issues, etc.
	if IsTransportRetryable(err) {
		return true
	}

	return false
}

// IsTransportRetryable reports whether a transport-level error is likely transient and worth retrying.
func IsTransportRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Respect caller context.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Timeouts (connect/read/write).
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}

	// DNS temporary/timeout.
	var de *net.DNSError
	if errors.As(err, &de) && (de.IsTemporary || de.Timeout()) {
		return true
	}

	// Some third-party types still expose Temporary() despite deprecation.
	type temporary interface{ Temporary() bool }
	var te temporary
	if errors.As(err, &te) && te.Temporary() {
		return true
	}

	// Mid-stream disconnects.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// Syscall-level conditions (reachable through *os.SyscallError, *net.OpError, *url.Error, etc.).
	var se syscall.Errno
	if errors.As(err, &se) {
		switch se {
		case syscall.ECONNRESET, // connection reset by peer
			syscall.ECONNABORTED, // connection aborted
			syscall.ECONNREFUSED, // no listener / port closed
			syscall.ETIMEDOUT,    // timeout
			syscall.EHOSTUNREACH, // host unreachable
			syscall.ENETUNREACH,  // network unreachable
			syscall.EPIPE:        // broken pipe
			return true
		}
	}

	// Prefer to treat *url.Error with Timeout/Temporary as retryable as well.
	var ue *url.Error
	if errors.As(err, &ue) {
		// url.Error often wraps net.Error; Timeout() is common here.
		if ue.Timeout() {
			return true
		}
	}

	return false
}

func AlwaysRetry(_ error) bool {
	return true
}

// RunOnError returns true, if an error is found.
func RunOnError(err error) bool {
	if err != nil {
		return true
	}
	return false
}

var RetriableErrorFn = func(err error) bool {
	if err != nil {
		return IsRetryableError(err)
	}
	return false
}
