package tasks

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

type failingStore struct{}

func (failingStore) Create(Task) (Task, error) {
	return Task{}, errors.New("disk on fire: secret-detail")
}

func post(t *testing.T, store Store, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewCreateHandler(store, func() time.Time { return fixedNow })
	req := httptest.NewRequest("POST", "/tasks", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type envelope struct {
	Error struct {
		Code    string
		Message string
		Fields  map[string]string
	}
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var e envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return e
}

func TestCreateSuccess(t *testing.T) {
	store := &MemStore{}
	rec := post(t, store, `{"title":"  Write docs  ","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":3}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if loc := rec.Header().Get("Location"); loc != "/tasks/1" {
		t.Errorf("Location = %q", loc)
	}
	if ct := rec.Result().Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	want := `{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":3,"created_at":"2030-01-01T09:00:00Z"}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("body\n got %s\nwant %s", got, want)
	}
	if len(store.Tasks) != 1 || store.Tasks[0].Title != "Write docs" {
		t.Errorf("store = %+v", store.Tasks)
	}
}

func TestCreateDefaultsAndEmptyTagsEncodeAsArray(t *testing.T) {
	rec := post(t, &MemStore{}, `{"title":"x"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"tags":[]`) || !strings.Contains(body, `"status":"todo"`) ||
		!strings.Contains(body, `"due_date":null`) || !strings.Contains(body, `"owner_id":null`) {
		t.Errorf("body = %s", body)
	}
}

func TestCreateMalformedBodiesAre400AndNeverReachTheStore(t *testing.T) {
	bodies := map[string]string{
		"empty":          ``,
		"truncated":      `{"title":`,
		"not an object":  `["x"]`,
		"wrong type":     `{"title":42}`,
		"unknown field":  `{"title":"x","id":99}`,
		"trailing data":  `{"title":"x"} {"title":"y"}`,
		"trailing junk":  `{"title":"x"} oops`,
		"tags wrong typ": `{"title":"x","tags":"docs"}`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			store := &MemStore{}
			rec := post(t, store, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if e := decodeEnvelope(t, rec); e.Error.Code != "bad_request" || e.Error.Message == "" {
				t.Errorf("envelope = %+v", e)
			}
			if len(store.Tasks) != 0 {
				t.Error("store was called")
			}
		})
	}
}

func TestCreateOversizedBodyIs413(t *testing.T) {
	store := &MemStore{}
	rec := post(t, store, `{"title":"`+strings.Repeat("a", 70<<10)+`"}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
	if e := decodeEnvelope(t, rec); e.Error.Code != "payload_too_large" {
		t.Errorf("code = %q", e.Error.Code)
	}
	if len(store.Tasks) != 0 {
		t.Error("store was called")
	}
}

func TestCreateValidationIs422WithEveryField(t *testing.T) {
	elevenTags := `["a","b","c","d","e","f","g","h","i","j","K"]`
	tests := []struct {
		name string
		body string
		want map[string]string
	}{
		{"blank title", `{"title":"   "}`, map[string]string{"title": "must be 1-200 characters"}},
		{"long title", `{"title":"` + strings.Repeat("a", 201) + `"}`, map[string]string{"title": "must be 1-200 characters"}},
		{"bad status", `{"title":"x","status":"blocked"}`, map[string]string{"status": "must be one of todo, in_progress, done"}},
		{"not a date", `{"title":"x","due_date":"31/01/2030"}`, map[string]string{"due_date": "must be a date in YYYY-MM-DD format"}},
		{"impossible date", `{"title":"x","due_date":"2030-02-30"}`, map[string]string{"due_date": "must be a date in YYYY-MM-DD format"}},
		{"uppercase tag", `{"title":"x","tags":["Docs"]}`, map[string]string{"tags": "each tag must be 1-30 lowercase characters and unique"}},
		{"empty tag", `{"title":"x","tags":[""]}`, map[string]string{"tags": "each tag must be 1-30 lowercase characters and unique"}},
		{"long tag", `{"title":"x","tags":["` + strings.Repeat("a", 31) + `"]}`, map[string]string{"tags": "each tag must be 1-30 lowercase characters and unique"}},
		{"duplicate tag", `{"title":"x","tags":["a","a"]}`, map[string]string{"tags": "each tag must be 1-30 lowercase characters and unique"}},
		{"too many tags", `{"title":"x","tags":` + elevenTags + `}`, map[string]string{"tags": "at most 10 tags"}},
		{"owner zero", `{"title":"x","owner_id":0}`, map[string]string{"owner_id": "must be a positive integer"}},
		{"everything at once", `{"title":"","status":"nope","due_date":"x","tags":["A"],"owner_id":-1}`, map[string]string{
			"title":    "must be 1-200 characters",
			"status":   "must be one of todo, in_progress, done",
			"due_date": "must be a date in YYYY-MM-DD format",
			"tags":     "each tag must be 1-30 lowercase characters and unique",
			"owner_id": "must be a positive integer",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &MemStore{}
			rec := post(t, store, tt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (body %s)", rec.Code, rec.Body)
			}
			e := decodeEnvelope(t, rec)
			if e.Error.Code != "validation_failed" {
				t.Errorf("code = %q", e.Error.Code)
			}
			if len(e.Error.Fields) != len(tt.want) {
				t.Fatalf("fields = %v, want %v", e.Error.Fields, tt.want)
			}
			for k, v := range tt.want {
				if e.Error.Fields[k] != v {
					t.Errorf("fields[%q] = %q, want %q", k, e.Error.Fields[k], v)
				}
			}
			if len(store.Tasks) != 0 {
				t.Error("store was called for invalid input")
			}
		})
	}
}

func TestCreateAcceptsBoundaryValues(t *testing.T) {
	tags := make([]string, 10)
	for i := range tags {
		tags[i] = strings.Repeat("t", i+1)
	}
	tagsJSON, _ := json.Marshal(tags)
	bodies := []string{
		`{"title":"` + strings.Repeat("é", 200) + `"}`, // 200 runes, 400 bytes
		`{"title":"x","status":"in_progress","due_date":"2030-02-28"}`,
		`{"title":"x","tags":` + string(tagsJSON) + `}`,
		`{"title":"x","tags":["` + strings.Repeat("a", 30) + `"]}`,
		`{"title":"x","owner_id":1,"due_date":null}`,
	}
	for _, body := range bodies {
		if rec := post(t, &MemStore{}, body); rec.Code != http.StatusCreated {
			t.Errorf("body %.60s...: status = %d, body %s", body, rec.Code, rec.Body)
		}
	}
}

func TestCreateNormalisesBeforeStoring(t *testing.T) {
	store := &MemStore{}
	post(t, store, `{"title":"  padded  "}`)
	if len(store.Tasks) != 1 {
		t.Fatal("task not stored")
	}
	got := store.Tasks[0]
	if got.Title != "padded" || got.Status != "todo" || got.Tags == nil || !slices.Equal(got.Tags, []string{}) {
		t.Errorf("stored = %+v", got)
	}
	if !got.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want the injected clock %v", got.CreatedAt, fixedNow)
	}
}

func TestCreateStoreFailureIs500WithoutDetails(t *testing.T) {
	rec := post(t, failingStore{}, `{"title":"x"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error.Code != "internal" || e.Error.Message != "internal server error" {
		t.Errorf("envelope = %+v", e)
	}
	if strings.Contains(rec.Body.String(), "secret-detail") {
		t.Error("store error leaked to the client")
	}
}
