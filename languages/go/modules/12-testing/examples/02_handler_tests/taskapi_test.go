package taskapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// do runs one request against h in memory: no port, no goroutine, no network.
func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("body is not the expected JSON: %v\n%s", err, rec.Body.String())
	}
	return v
}

type errorEnvelope struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"valid", `{"title":"Write docs"}`, http.StatusCreated, "", ""},
		{"malformed JSON", `{"title":`, http.StatusBadRequest, "bad_request", ""},
		{"unknown field", `{"title":"x","colour":"red"}`, http.StatusBadRequest, "bad_request", ""},
		{"blank title", `{"title":"  "}`, http.StatusUnprocessableEntity, "validation_failed", "title"},
		{"bad status", `{"title":"x","status":"later"}`, http.StatusUnprocessableEntity, "validation_failed", "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, New(), "POST", "/tasks", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			if tt.wantCode == "" {
				if got := rec.Header().Get("Location"); got != "/tasks/1" {
					t.Errorf("Location = %q", got)
				}
				if task := decode[Task](t, rec); task != (Task{ID: 1, Title: "Write docs", Status: "todo"}) {
					t.Errorf("task = %+v", task)
				}
				return
			}
			env := decode[errorEnvelope](t, rec)
			if env.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", env.Error.Code, tt.wantCode)
			}
			if _, ok := env.Error.Fields[tt.wantField]; tt.wantField != "" && !ok {
				t.Errorf("fields = %v, want an entry for %q", env.Error.Fields, tt.wantField)
			}
		})
	}
}

func TestGetAndNotFound(t *testing.T) {
	h := New()
	do(t, h, "POST", "/tasks", `{"title":"Write docs"}`)

	if rec := do(t, h, "GET", "/tasks/1", ""); rec.Code != http.StatusOK {
		t.Fatalf("GET /tasks/1 = %d", rec.Code)
	}
	for _, path := range []string{"/tasks/2", "/tasks/0", "/tasks/abc"} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != http.StatusNotFound || decode[errorEnvelope](t, rec).Error.Code != "not_found" {
			t.Errorf("GET %s = %d %s", path, rec.Code, rec.Body)
		}
	}
}

func TestListFiltersByStatus(t *testing.T) {
	h := New()
	for _, body := range []string{
		`{"title":"a"}`, `{"title":"b","status":"done"}`, `{"title":"c","status":"done"}`,
	} {
		do(t, h, "POST", "/tasks", body)
	}
	list := decode[struct {
		Items []Task `json:"items"`
		Total int    `json:"total"`
	}](t, do(t, h, "GET", "/tasks?status=done", ""))
	if list.Total != 2 || len(list.Items) != 2 || list.Items[0].Title != "b" {
		t.Fatalf("list = %+v", list)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := do(t, New(), "DELETE", "/tasks", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "POST") {
		t.Errorf("Allow = %q", allow)
	}
}

// One test over a real server proves the wiring that the recorder skips: TCP, the client, redirects, headers.
func TestOverRealServer(t *testing.T) {
	srv := httptest.NewServer(New())
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/tasks", "application/json", strings.NewReader(`{"title":"Ship it"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	get, err := http.Get(srv.URL + resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	body, _ := io.ReadAll(get.Body)
	if get.StatusCode != http.StatusOK || !strings.Contains(string(body), `"title":"Ship it"`) {
		t.Fatalf("GET = %d %s", get.StatusCode, body)
	}
}
