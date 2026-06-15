package http

import (
	"github.com/ing-bank/golibs/pkg/errors"
)

var ErrHttpStatus = errors.ErrHTTPStatus

var ErrBadRequest = errors.ErrBadRequest
var ErrUnauthorized = errors.ErrUnauthorized
var ErrPaymentRequired = errors.ErrPaymentRequired
var ErrForbidden = errors.ErrForbidden
var ErrNotFound = errors.ErrNotFound
var ErrMethodNotAllowed = errors.ErrMethodNotAllowed
var ErrNotAcceptable = errors.ErrNotAcceptable
var ErrProxyAuthRequired = errors.ErrProxyAuthRequired
var ErrRequestTimeout = errors.ErrRequestTimeout
var ErrConflict = errors.ErrConflict
var ErrGone = errors.ErrGone
var ErrLocked = errors.ErrLocked
var ErrTooEarly = errors.ErrTooEarly
var ErrTooManyRequests = errors.ErrTooManyRequests

var ErrInternalServerError = errors.ErrInternalServerError
var ErrNotImplemented = errors.ErrNotImplemented
var ErrBadGateway = errors.ErrBadGateway
var ErrServiceUnavailable = errors.ErrServiceUnavailable
var ErrGatewayTimeout = errors.ErrGatewayTimeout

var ErrBuildRequest = errors.ErrBuildRequest
var ErrEnvironmentVariable = errors.ErrEnvironmentVariable
var ErrInvalidResponse = errors.ErrInvalidResponse
var ErrFailedRequestOption = errors.ErrFailedRequestOption
