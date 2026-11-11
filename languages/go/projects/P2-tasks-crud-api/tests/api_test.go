package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"webdev/projects/P2-tasks-crud-api/starter"
)

type api struct {
	t   *testing.T
	url string
}

func newAPI(t *testing.T) *api {
	t.Helper()
	db, err := starter.OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h, err := starter.New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &api{t: t, url: srv.URL}
}

// do sends body (a string is sent as-is, anything else is JSON-encoded) and
// returns the response and its raw body.
func (a *api) do(method, path string, body any) (*http.Response, string) {
	a.t.Helper()
	var rdr io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rdr = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		rdr = strings.NewReader(string(raw))
	}
	req, err := http.NewRequest(method, a.url+path, rdr)
	if err != nil {
		a.t.Fatal(err)
	}
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		a.t.Fatal(err)
	}
	return resp, string(raw)
}

type taskJSON struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	DueDate   *string  `json:"due_date"`
	Tags      []string `json:"tags"`
	OwnerID   *int64   `json:"owner_id"`
	CreatedAt string   `json:"created_at"`
}

type listJSON struct {
	Items  []taskJSON `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

type errorJSON struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

func decode[T any](t *testing.T, raw string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode %q: %v", raw, err)
	}
	return v
}

func (a *api) create(body any) taskJSON {
	a.t.Helper()
	resp, raw := a.do("POST", "/tasks", body)
	if resp.StatusCode != http.StatusCreated {
		a.t.Fatalf("create %v: status %d, body %s", body, resp.StatusCode, raw)
	}
	return decode[taskJSON](a.t, raw)
}

func (a *api) list(query string) listJSON {
	a.t.Helper()
	resp, raw := a.do("GET", "/tasks"+query, nil)
	if resp.StatusCode != http.StatusOK {
		a.t.Fatalf("list %s: status %d, body %s", query, resp.StatusCode, raw)
	}
	return decode[listJSON](a.t, raw)
}

func titles(l listJSON) []string {
	out := make([]string, 0, len(l.Items))
	for _, it := range l.Items {
		out = append(out, it.Title)
	}
	return out
}

func assertError(t *testing.T, resp *http.Response, raw string, status int, code string) errorJSON {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d (body %s)", resp.StatusCode, status, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	e := decode[errorJSON](t, raw)
	if e.Error.Code != code {
		t.Errorf("error code = %q, want %q", e.Error.Code, code)
	}
	return e
}

// --- create ---------------------------------------------------------------

func TestCreateReturns201WithLocationAndDefaults(t *testing.T) {
	a := newAPI(t)
	resp, raw := a.do("POST", "/tasks", map[string]any{"title": "Write docs"})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	task := decode[taskJSON](t, raw)
	if loc := resp.Header.Get("Location"); loc != fmt.Sprintf("/tasks/%d", task.ID) || task.ID == 0 {
		t.Errorf("Location = %q for id %d", loc, task.ID)
	}
	if task.Title != "Write docs" || task.Status != "todo" || task.DueDate != nil || task.OwnerID != nil {
		t.Errorf("unexpected defaults: %+v", task)
	}
	if task.Tags == nil || len(task.Tags) != 0 {
		t.Errorf("tags = %#v, want an empty array", task.Tags)
	}
	if !strings.Contains(raw, `"tags":[]`) || !strings.Contains(raw, `"due_date":null`) || !strings.Contains(raw, `"owner_id":null`) {
		t.Errorf("wire format must carry [] and null explicitly: %s", raw)
	}
}

func TestCreateStoresAllFieldsAndTimestampIsRecentUTC(t *testing.T) {
	a := newAPI(t)
	task := a.create(map[string]any{
		"title": "Ship it", "status": "in_progress", "due_date": "2030-01-31",
		"tags": []string{"docs", "p1"}, "owner_id": 7,
	})
	if task.Status != "in_progress" || task.DueDate == nil || *task.DueDate != "2030-01-31" ||
		!slices.Equal(task.Tags, []string{"docs", "p1"}) || task.OwnerID == nil || *task.OwnerID != 7 {
		t.Errorf("unexpected task: %+v", task)
	}
	created, err := time.Parse(time.RFC3339, task.CreatedAt)
	if err != nil || !strings.HasSuffix(task.CreatedAt, "Z") {
		t.Fatalf("created_at = %q, want RFC 3339 UTC", task.CreatedAt)
	}
	if d := time.Since(created); d < -time.Minute || d > time.Minute {
		t.Errorf("created_at %v is not close to now", created)
	}
}

func TestCreateAssignsIncreasingIDsAndTrimsTitle(t *testing.T) {
	a := newAPI(t)
	first := a.create(map[string]any{"title": "  padded  "})
	second := a.create(map[string]any{"title": "b"})
	if first.Title != "padded" {
		t.Errorf("title = %q, want it trimmed", first.Title)
	}
	if second.ID <= first.ID {
		t.Errorf("ids %d then %d should increase", first.ID, second.ID)
	}
}

// --- validation and malformed input ---------------------------------------

func TestCreateRejectsInvalidDataWith422AndFieldMessages(t *testing.T) {
	tags11 := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}
	tests := []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"missing title", map[string]any{}, "title"},
		{"blank title", map[string]any{"title": "   "}, "title"},
		{"title over 200 chars", map[string]any{"title": strings.Repeat("a", 201)}, "title"},
		{"unknown status", map[string]any{"title": "x", "status": "finished"}, "status"},
		{"impossible date", map[string]any{"title": "x", "due_date": "2030-02-30"}, "due_date"},
		{"wrong date format", map[string]any{"title": "x", "due_date": "31/01/2030"}, "due_date"},
		{"more than 10 tags", map[string]any{"title": "x", "tags": tags11}, "tags"},
		{"empty tag", map[string]any{"title": "x", "tags": []string{""}}, "tags"},
		{"tag over 30 chars", map[string]any{"title": "x", "tags": []string{strings.Repeat("t", 31)}}, "tags"},
		{"uppercase tag", map[string]any{"title": "x", "tags": []string{"Docs"}}, "tags"},
		{"duplicate tag", map[string]any{"title": "x", "tags": []string{"a", "a"}}, "tags"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newAPI(t)
			resp, raw := a.do("POST", "/tasks", tc.body)
			e := assertError(t, resp, raw, 422, "validation_failed")
			if e.Error.Fields[tc.field] == "" {
				t.Errorf("fields = %v, want a message for %q", e.Error.Fields, tc.field)
			}
		})
	}
}

func TestCreateAcceptsBoundaryValues(t *testing.T) {
	a := newAPI(t)
	ten := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", strings.Repeat("j", 30)}
	a.create(map[string]any{"title": strings.Repeat("é", 200), "tags": ten})
}

func TestMalformedJSONIs400BadRequest(t *testing.T) {
	a := newAPI(t)
	for name, body := range map[string]string{"garbage": `{"title":`, "not an object": `[1,2]`, "empty body": ``} {
		t.Run(name, func(t *testing.T) {
			resp, raw := a.do("POST", "/tasks", body)
			assertError(t, resp, raw, 400, "bad_request")
		})
	}
}

func TestOversizedBodyIs413(t *testing.T) {
	a := newAPI(t)
	big := `{"title":"` + strings.Repeat("a", 2<<20) + `"}`
	resp, raw := a.do("POST", "/tasks", big)
	assertError(t, resp, raw, 413, "payload_too_large")
}

// --- read / delete ----------------------------------------------------------

func TestGetReturnsTheStoredTask(t *testing.T) {
	a := newAPI(t)
	created := a.create(map[string]any{"title": "find me", "tags": []string{"x"}})
	resp, raw := a.do("GET", fmt.Sprintf("/tasks/%d", created.ID), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	if got := decode[taskJSON](t, raw); got.ID != created.ID || got.Title != "find me" || !slices.Equal(got.Tags, []string{"x"}) || got.CreatedAt != created.CreatedAt {
		t.Errorf("got %+v, want %+v", got, created)
	}
}

func TestUnknownOrMalformedIDIs404WithEnvelope(t *testing.T) {
	a := newAPI(t)
	for _, id := range []string{"12345", "abc", "0", "-1"} {
		t.Run(id, func(t *testing.T) {
			resp, raw := a.do("GET", "/tasks/"+id, nil)
			assertError(t, resp, raw, 404, "not_found")
		})
	}
}

func TestDeleteReturns204ThenTaskIsGone(t *testing.T) {
	a := newAPI(t)
	task := a.create(map[string]any{"title": "temp"})
	path := fmt.Sprintf("/tasks/%d", task.ID)
	if resp, raw := a.do("DELETE", path, nil); resp.StatusCode != 204 || raw != "" {
		t.Fatalf("delete: status %d body %q, want 204 and no body", resp.StatusCode, raw)
	}
	resp, raw := a.do("GET", path, nil)
	assertError(t, resp, raw, 404, "not_found")
}

// --- PUT --------------------------------------------------------------------

func TestPutReplacesEveryWritableField(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "old", "status": "done", "due_date": "2030-05-05", "tags": []string{"a"}, "owner_id": 3})
	resp, raw := a.do("PUT", fmt.Sprintf("/tasks/%d", orig.ID), map[string]any{"title": "new"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	got := decode[taskJSON](t, raw)
	if got.Title != "new" || got.Status != "todo" || got.DueDate != nil || len(got.Tags) != 0 || got.OwnerID != nil {
		t.Errorf("omitted fields must reset to defaults: %+v", got)
	}
	if got.ID != orig.ID || got.CreatedAt != orig.CreatedAt {
		t.Errorf("id and created_at must be kept: %+v vs %+v", got, orig)
	}
}

func TestPutInvalidDataIs422AndLeavesTaskUnchanged(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "keep"})
	path := fmt.Sprintf("/tasks/%d", orig.ID)
	resp, raw := a.do("PUT", path, map[string]any{"title": ""})
	assertError(t, resp, raw, 422, "validation_failed")
	_, raw = a.do("GET", path, nil)
	if got := decode[taskJSON](t, raw); got.Title != "keep" {
		t.Errorf("title = %q after a rejected PUT", got.Title)
	}
}

// --- PATCH ------------------------------------------------------------------

func TestPatchChangesOnlyTheGivenFields(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "t", "status": "todo", "due_date": "2030-01-01", "tags": []string{"a"}, "owner_id": 5})
	resp, raw := a.do("PATCH", fmt.Sprintf("/tasks/%d", orig.ID), map[string]any{"status": "done"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	got := decode[taskJSON](t, raw)
	if got.Status != "done" {
		t.Errorf("status = %q, want done", got.Status)
	}
	if got.Title != "t" || got.DueDate == nil || *got.DueDate != "2030-01-01" || !slices.Equal(got.Tags, []string{"a"}) || got.OwnerID == nil || *got.OwnerID != 5 {
		t.Errorf("absent fields must stay unchanged: %+v", got)
	}
}

func TestPatchNullDueDateClearsIt(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "t", "due_date": "2030-01-01"})
	resp, raw := a.do("PATCH", fmt.Sprintf("/tasks/%d", orig.ID), `{"due_date":null}`)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, body %s", resp.StatusCode, raw)
	}
	if got := decode[taskJSON](t, raw); got.DueDate != nil || got.Title != "t" {
		t.Errorf("due_date must be cleared and title kept: %+v", got)
	}
}

func TestPatchIgnoresOwnerID(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "t", "owner_id": 1})
	_, raw := a.do("PATCH", fmt.Sprintf("/tasks/%d", orig.ID), map[string]any{"owner_id": 2})
	if got := decode[taskJSON](t, raw); got.OwnerID == nil || *got.OwnerID != 1 {
		t.Errorf("owner_id must not be patchable: %+v", got)
	}
}

func TestPatchValidatesTheMergedResult(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "t"})
	path := fmt.Sprintf("/tasks/%d", orig.ID)
	resp, raw := a.do("PATCH", path, map[string]any{"status": "bogus"})
	e := assertError(t, resp, raw, 422, "validation_failed")
	if e.Error.Fields["status"] == "" {
		t.Errorf("fields = %v", e.Error.Fields)
	}
	_, raw = a.do("GET", path, nil)
	if got := decode[taskJSON](t, raw); got.Status != "todo" {
		t.Errorf("status = %q after a rejected PATCH", got.Status)
	}
}

func TestPatchMalformedBodyIs400(t *testing.T) {
	a := newAPI(t)
	orig := a.create(map[string]any{"title": "t"})
	resp, raw := a.do("PATCH", fmt.Sprintf("/tasks/%d", orig.ID), `{"title":`)
	assertError(t, resp, raw, 400, "bad_request")
}

func TestWritingToAnUnknownIDIs404ForPutPatchAndDelete(t *testing.T) {
	a := newAPI(t)
	for _, method := range []string{"PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			resp, raw := a.do(method, "/tasks/42", map[string]any{"title": "x"})
			assertError(t, resp, raw, 404, "not_found")
		})
	}
}

// --- list -------------------------------------------------------------------

func TestListOnEmptyStoreHasEmptyItemsArrayAndDefaults(t *testing.T) {
	a := newAPI(t)
	resp, raw := a.do("GET", "/tasks", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(raw, `"items":[]`) {
		t.Errorf("items must be [] not null: %s", raw)
	}
	l := decode[listJSON](t, raw)
	if l.Total != 0 || l.Limit != 20 || l.Offset != 0 {
		t.Errorf("envelope = %+v, want total 0 limit 20 offset 0", l)
	}
}

func TestListDefaultsToNewestFirstAndSortCanReverseIt(t *testing.T) {
	a := newAPI(t)
	for _, title := range []string{"first", "second", "third"} {
		a.create(map[string]any{"title": title})
	}
	if got := titles(a.list("")); !slices.Equal(got, []string{"third", "second", "first"}) {
		t.Errorf("default order = %v", got)
	}
	if got := titles(a.list("?sort=-created_at")); !slices.Equal(got, []string{"third", "second", "first"}) {
		t.Errorf("sort=-created_at = %v", got)
	}
	if got := titles(a.list("?sort=created_at")); !slices.Equal(got, []string{"first", "second", "third"}) {
		t.Errorf("sort=created_at = %v", got)
	}
}

func TestListSortsByDueDateWithMissingDatesLast(t *testing.T) {
	a := newAPI(t)
	a.create(map[string]any{"title": "none"})
	a.create(map[string]any{"title": "late", "due_date": "2031-01-01"})
	a.create(map[string]any{"title": "early", "due_date": "2030-01-01"})
	if got := titles(a.list("?sort=due_date")); !slices.Equal(got, []string{"early", "late", "none"}) {
		t.Errorf("sort=due_date = %v", got)
	}
	if got := titles(a.list("?sort=-due_date")); !slices.Equal(got, []string{"late", "early", "none"}) {
		t.Errorf("sort=-due_date = %v", got)
	}
}

func TestListFiltersByStatusAndTag(t *testing.T) {
	a := newAPI(t)
	a.create(map[string]any{"title": "a", "status": "done", "tags": []string{"x", "y"}})
	a.create(map[string]any{"title": "b", "status": "todo", "tags": []string{"y"}})
	a.create(map[string]any{"title": "c", "status": "done"})

	if l := a.list("?status=done"); l.Total != 2 || !slices.Equal(titles(l), []string{"c", "a"}) {
		t.Errorf("status=done -> %v total %d", titles(l), l.Total)
	}
	if l := a.list("?tag=y"); l.Total != 2 || !slices.Equal(titles(l), []string{"b", "a"}) {
		t.Errorf("tag=y -> %v total %d", titles(l), l.Total)
	}
	if l := a.list("?tag=x&status=done"); !slices.Equal(titles(l), []string{"a"}) {
		t.Errorf("tag=x&status=done -> %v", titles(l))
	}
	if l := a.list("?tag=missing"); l.Total != 0 || len(l.Items) != 0 {
		t.Errorf("tag=missing -> %+v", l)
	}
}

func TestListPaginatesAndReportsTotal(t *testing.T) {
	a := newAPI(t)
	for i := 1; i <= 5; i++ {
		a.create(map[string]any{"title": fmt.Sprintf("t%d", i)})
	}
	l := a.list("?sort=created_at&limit=2&offset=2")
	if !slices.Equal(titles(l), []string{"t3", "t4"}) || l.Total != 5 || l.Limit != 2 || l.Offset != 2 {
		t.Errorf("page = %v %+v", titles(l), l)
	}
	if l := a.list("?limit=2&offset=10"); len(l.Items) != 0 || l.Total != 5 {
		t.Errorf("offset past the end = %+v", l)
	}
}

func TestListClampsLimitTo100(t *testing.T) {
	a := newAPI(t)
	a.create(map[string]any{"title": "x"})
	if l := a.list("?limit=1000"); l.Limit != 100 {
		t.Errorf("limit = %d, want 100", l.Limit)
	}
}

func TestListRejectsBadQueryValuesWith400(t *testing.T) {
	for _, q := range []string{"?status=bogus", "?limit=abc", "?limit=0", "?limit=-5", "?offset=-1", "?offset=x", "?sort=title"} {
		t.Run(q, func(t *testing.T) {
			a := newAPI(t)
			resp, raw := a.do("GET", "/tasks"+q, nil)
			assertError(t, resp, raw, 400, "bad_request")
		})
	}
}

func TestSQLInjectionInFiltersIsHarmless(t *testing.T) {
	a := newAPI(t)
	a.create(map[string]any{"title": "safe", "tags": []string{"x"}})
	l := a.list("?tag=" + "x'%20OR%20'1'='1")
	if l.Total != 0 {
		t.Errorf("injection matched %d tasks", l.Total)
	}
	if a.list("").Total != 1 {
		t.Error("the table must be intact")
	}
}
