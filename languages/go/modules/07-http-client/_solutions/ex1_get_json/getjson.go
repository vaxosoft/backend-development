// Package getjson is exercise 1: a safe JSON GET helper.
package getjson

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &StatusError{Code: resp.StatusCode, Body: string(body)}
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
