package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]map[string]any {
	t.Helper()
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not a JSON object of objects: %v\n%q", err, rec.Body.String())
	}
	return body
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]int{"id": 7})

	res := rec.Result()
	if res.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q (headers must be set before WriteHeader)", ct)
	}
	var got map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got["id"] != 7 {
		t.Errorf("body = %q, %v", rec.Body.String(), err)
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 422, "validation_failed", "invalid task", map[string]string{"title": "must be 1-200 characters"})

	res := rec.Result()
	if res.StatusCode != 422 || res.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("got %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	e := decodeBody(t, rec)["error"]
	if e["code"] != "validation_failed" || e["message"] != "invalid task" {
		t.Fatalf("error = %v", e)
	}
	fields, _ := e["fields"].(map[string]any)
	if fields["title"] != "must be 1-200 characters" {
		t.Fatalf("fields = %v", e["fields"])
	}
}

func TestWriteErrorOmitsEmptyFields(t *testing.T) {
	for _, fields := range []map[string]string{nil, {}} {
		rec := httptest.NewRecorder()
		WriteError(rec, 404, "not_found", "no such task", fields)
		if _, present := decodeBody(t, rec)["error"]["fields"]; present {
			t.Errorf("fields = %#v should be omitted, body %q", fields, rec.Body.String())
		}
	}
}

func TestWriteErr(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"not found", ErrNotFound, 404, "not_found", "resource not found"},
		{"wrapped not found", fmt.Errorf("get task 9: %w", ErrNotFound), 404, "not_found", "resource not found"},
		{"bad request keeps message", fmt.Errorf("limit: %w", ErrBadRequest), 400, "bad_request", "limit: bad request"},
		{"too large", fmt.Errorf("read: %w", &http.MaxBytesError{Limit: 10}), 413, "payload_too_large", "request body too large"},
		{"unknown error", errors.New("pq: password=hunter2 refused"), 500, "internal", "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteErr(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			e := decodeBody(t, rec)["error"]
			if e["code"] != tt.wantCode || e["message"] != tt.wantMsg {
				t.Fatalf("error = %v, want code %q message %q", e, tt.wantCode, tt.wantMsg)
			}
			if strings.Contains(rec.Body.String(), "hunter2") {
				t.Fatal("internal error text leaked to the client")
			}
		})
	}
}

func TestWriteErrValidation(t *testing.T) {
	err := fmt.Errorf("create: %w", &ValidationError{Fields: map[string]string{"title": "required", "tags": "too many"}})
	rec := httptest.NewRecorder()
	WriteErr(rec, err)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	e := decodeBody(t, rec)["error"]
	fields, _ := e["fields"].(map[string]any)
	if e["code"] != "validation_failed" || e["message"] != "invalid request" || len(fields) != 2 || fields["title"] != "required" {
		t.Fatalf("error = %v", e)
	}
}
