// Package httpcheck is exercise 2: a small test-helper package for HTTP handler tests.
//
// Run the tests: go test ./modules/12-testing/exercises/ex2_http_check/
package httpcheck

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
)

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
	t.Helper()
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewReader([]byte(b))
	case []byte:
		reader = bytes.NewReader(b)
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("encode request body: %v", err)
			return Response{}
		}
		reader = bytes.NewReader(encoded)
		contentType = "application/json"
	}
	req := httptest.NewRequest(method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return Response{Status: res.StatusCode, Header: res.Header, Body: data}
}

// JSON decodes the body into v and stops the test with t.Fatalf when it is not valid JSON for v.
func (r Response) JSON(t T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("response body is not the expected JSON: %v (body %q)", err, r.Body)
	}
}

// ExpectStatus reports a (non-fatal) error when the status differs, and returns r for chaining.
func (r Response) ExpectStatus(t T, want int) Response {
	t.Helper()
	if r.Status != want {
		t.Errorf("status = %d, want %d (body %q)", r.Status, want, r.Body)
	}
	return r
}

// ExpectHeader reports a (non-fatal) error when the header value differs, and returns r.
func (r Response) ExpectHeader(t T, key, want string) Response {
	t.Helper()
	if got := r.Header.Get(key); got != want {
		t.Errorf("header %s = %q, want %q", key, got, want)
	}
	return r
}

// ExpectError checks the shared error envelope {"error":{"code":"..."}}. Each problem is reported
// once with t.Errorf: wrong status, wrong Content-Type (must be application/json), wrong code.
// A body that is not valid JSON counts as one problem, and the code is then not checked.
func (r Response) ExpectError(t T, status int, code string) Response {
	t.Helper()
	r.ExpectStatus(t, status)
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.Body, &env); err != nil {
		t.Errorf("body is not the error envelope: %v", err)
		return r
	}
	if env.Error.Code != code {
		t.Errorf("error code = %q, want %q", env.Error.Code, code)
	}
	return r
}
