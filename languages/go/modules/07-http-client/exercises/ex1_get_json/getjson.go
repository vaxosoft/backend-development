// Package getjson is exercise 1: a safe JSON GET helper.
package getjson

import (
	"context"
	"fmt"
	"net/http"
)

// StatusError reports a non-2xx response.
type StatusError struct {
	Code int
	// Body holds at most the first 1024 bytes of the response body.
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("unexpected status %d", e.Code) }

// GetJSON performs GET url with the "Accept: application/json" header using c and decodes the
// JSON body into out.
//
//   - A non-2xx status returns a *StatusError (with the start of the body).
//   - Transport errors are returned wrapped, so errors.Is/As still work.
//   - The response body must always be closed, on every path.
func GetJSON(ctx context.Context, c *http.Client, url string, out any) error {
	panic("TODO")
}
