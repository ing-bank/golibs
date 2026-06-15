package response

import (
	"encoding/json"
	"fmt"
	gohttp "net/http"

	"github.com/ing-bank/golibs/pkg/errors"
)

// Data captures a HTTP response
type Data struct {
	Headers gohttp.Header
	Raw     []byte // Raw payload
	Status  int    // Status code
	Err     error  // Transport, status, or payload parsing errors
}

// Parse unmarshals JSON into response if there was no error yet
func (r *Data) Parse(response any) *Data {
	if r.Err == nil {
		r.Err = json.Unmarshal(r.Raw, response)
	}
	return r
}

// MustParse tries to parse the payload data if it is present
func (r *Data) MustParse(response any) (*Data, error) {
	if r.Raw == nil || len(r.Raw) == 0 || !json.Valid(r.Raw) {
		return r, fmt.Errorf("%w: failed to parse request payload: '%s'", errors.ErrInvalidResponse, r.Raw)
	}
	return r, json.Unmarshal(r.Raw, response)
}

// IsOK returns true if there is no error and the HTTP status code is 'acceptable'
func (r *Data) IsOK() bool {
	return r.Err == nil && errors.StatusCodeAsErr(r.Status) == nil
}

// Error returns transport, status or payload parsing errors, if any
func (r *Data) Error() error {
	if r.IsOK() {
		return nil
	}

	if r.Err != nil {
		return r.Err
	}

	return errors.StatusCodeAsErr(r.Status)
}
