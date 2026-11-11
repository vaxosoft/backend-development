// Package httpcheck is exercise 2: a small test-helper package for HTTP handler tests.
//
// Run the tests: go test ./modules/12-testing/exercises/ex2_http_check/
package httpcheck

import "net/http"

// T is the part of *testing.T the helpers need. Declaring it ourselves lets the package's own
// tests pass a recording fake and check how the helpers behave when an assertion fails.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// Response is a recorded handler response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Do calls h in memory (httptest.NewRecorder, no network) and returns the response.
//
// body may be nil (no body), a string or []byte (sent as is), or any other value, which is
// encoded as JSON and sent with Content-Type: application/json. A value that cannot be encoded
// stops the test with t.Fatalf. Every helper in this package must call t.Helper() first.
func Do(t T, h http.Handler, method, target string, body any) Response {
	return Response{}
}

// JSON decodes the body into v and stops the test with t.Fatalf when it is not valid JSON for v.
func (r Response) JSON(t T, v any) {
}

// ExpectStatus reports a (non-fatal) error when the status differs, and returns r for chaining.
func (r Response) ExpectStatus(t T, want int) Response {
	return r
}

// ExpectHeader reports a (non-fatal) error when the header value differs, and returns r.
func (r Response) ExpectHeader(t T, key, want string) Response {
	return r
}

// ExpectError checks the shared error envelope {"error":{"code":"..."}}. Each problem is reported
// once with t.Errorf: wrong status, wrong Content-Type (must be application/json), wrong code.
// A body that is not valid JSON counts as one problem, and the code is then not checked.
func (r Response) ExpectError(t T, status int, code string) Response {
	return r
}
