package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func do(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewMux().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string
	}{
		{"hello default", "GET", "/hello", 200, "Hello, world!\n"},
		{"hello name", "GET", "/hello/Ada", 200, "Hello, Ada!\n"},
		{"hello shout", "GET", "/hello/Ada?shout=true", 200, "HELLO, ADA!\n"},
		{"hello shout default name", "GET", "/hello?shout=true", 200, "HELLO, WORLD!\n"},
		{"shout other value ignored", "GET", "/hello/Ada?shout=no", 200, "Hello, Ada!\n"},
		{"name too long", "GET", "/hello/" + strings.Repeat("a", 51), 400, "name too long\n"},
		{"name at limit", "GET", "/hello/" + strings.Repeat("a", 50), 200, "Hello, " + strings.Repeat("a", 50) + "!\n"},
		{"task id", "GET", "/tasks/42", 200, "task 42\n"},
		{"task id not a number", "GET", "/tasks/abc", 400, "invalid id\n"},
		{"task id zero", "GET", "/tasks/0", 400, "invalid id\n"},
		{"task id negative", "GET", "/tasks/-3", 400, "invalid id\n"},
		{"file with slashes", "GET", "/files/a/b/c.txt", 200, "file: a/b/c.txt\n"},
		{"file single segment", "GET", "/files/readme.md", 200, "file: readme.md\n"},
		{"unknown path", "GET", "/nope", 404, "404 page not found\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, tt.method, tt.target)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestWrongMethodGets405WithAllowHeader(t *testing.T) {
	rec := do(t, "POST", "/hello/Ada")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("Allow = %q, want it to list GET", allow)
	}
}

func TestErrorResponsesDoNotContinueWritingTheGreeting(t *testing.T) {
	rec := do(t, "GET", "/hello/"+strings.Repeat("x", 80))
	if strings.Contains(rec.Body.String(), "Hello") {
		t.Fatalf("handler kept writing after http.Error (forgot to return?): %q", rec.Body.String())
	}
}

func TestRootIsNotRegistered(t *testing.T) {
	if rec := do(t, "GET", "/"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET / = %d, want 404", rec.Code)
	}
}
