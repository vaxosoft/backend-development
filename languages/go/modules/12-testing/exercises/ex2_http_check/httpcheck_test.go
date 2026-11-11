package httpcheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
)

// fakeT records what the helpers do to the test. Fatalf stops the code under test the way the
// real one does (runtime.Goexit), here with a panic that run recovers.
type fakeT struct {
	helpers int
	errors  []string
	fatals  []string
}

type stopTest struct{}

func (f *fakeT) Helper() { f.helpers++ }
func (f *fakeT) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatals = append(f.fatals, fmt.Sprintf(format, args...))
	panic(stopTest{})
}

func run(fn func(t *fakeT)) *fakeT {
	ft := &fakeT{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(stopTest); !ok {
					panic(r)
				}
			}
		}()
		fn(ft)
	}()
	return ft
}

// expect checks only counts, never message text: wording is not part of the contract.
func expect(t *testing.T, ft *fakeT, errs, fatals int) {
	t.Helper()
	if len(ft.errors) != errs || len(ft.fatals) != fatals {
		t.Errorf("errors = %d, fatals = %d; want %d and %d (%q %q)", len(ft.errors), len(ft.fatals), errs, fatals, ft.errors, ft.fatals)
	}
	if ft.helpers == 0 {
		t.Error("the helper never called t.Helper(), so failures would point at the helper instead of the caller")
	}
}

type seen struct {
	method, uri, contentType, body string
}

// echo records the request and answers with a fixed response.
func echo(rec *seen) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*rec = seen{r.Method, r.URL.RequestURI(), r.Header.Get("Content-Type"), string(b)}
		w.Header().Set("Location", "/tasks/1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1,"title":"a"}`)
	})
}

func TestDoEncodesJSONBodies(t *testing.T) {
	var s seen
	var resp Response
	ft := run(func(ft *fakeT) {
		resp = Do(ft, echo(&s), "POST", "/tasks?x=1", map[string]any{"title": "a"})
	})
	expect(t, ft, 0, 0)

	if s.method != "POST" || s.uri != "/tasks?x=1" || s.contentType != "application/json" {
		t.Errorf("handler saw %+v", s)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(s.body), &sent); err != nil || !reflect.DeepEqual(sent, map[string]any{"title": "a"}) {
		t.Errorf("handler received body %q", s.body)
	}
	if resp.Status != http.StatusCreated || resp.Header.Get("Location") != "/tasks/1" || string(resp.Body) != `{"id":1,"title":"a"}` {
		t.Errorf("response = %d %v %q", resp.Status, resp.Header, resp.Body)
	}
}

func TestDoSendsStringsAndBytesUntouched(t *testing.T) {
	for name, body := range map[string]any{"string": `{"title":`, "bytes": []byte(`{"title":`)} {
		t.Run(name, func(t *testing.T) {
			var s seen
			ft := run(func(ft *fakeT) { Do(ft, echo(&s), "PUT", "/tasks/1", body) })
			expect(t, ft, 0, 0)
			if s.body != `{"title":` || s.contentType != "" {
				t.Errorf("handler saw body %q, Content-Type %q: raw bodies must not be re-encoded or labelled", s.body, s.contentType)
			}
		})
	}
}

func TestDoWithoutBody(t *testing.T) {
	var s seen
	ft := run(func(ft *fakeT) { Do(ft, echo(&s), "GET", "/tasks", nil) })
	expect(t, ft, 0, 0)
	if s.method != "GET" || s.body != "" || s.contentType != "" {
		t.Errorf("handler saw %+v", s)
	}
}

func TestDoStopsTheTestWhenTheBodyCannotBeEncoded(t *testing.T) {
	var s seen
	ft := run(func(ft *fakeT) { Do(ft, echo(&s), "POST", "/tasks", make(chan int)) })
	expect(t, ft, 0, 1)
	if s.method != "" {
		t.Error("the handler must not be called after the failure")
	}
}

func TestJSON(t *testing.T) {
	r := Response{Status: 200, Body: []byte(`{"id":7,"title":"x"}`)}

	var got struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	ft := run(func(ft *fakeT) { r.JSON(ft, &got) })
	expect(t, ft, 0, 0)
	if got.ID != 7 || got.Title != "x" {
		t.Errorf("decoded %+v", got)
	}

	var wrongType []int
	expect(t, run(func(ft *fakeT) { r.JSON(ft, &wrongType) }), 0, 1)
	expect(t, run(func(ft *fakeT) { Response{Body: []byte("not json")}.JSON(ft, &got) }), 0, 1)
}

func TestExpectStatus(t *testing.T) {
	r := Response{Status: http.StatusCreated}
	var chained Response
	expect(t, run(func(ft *fakeT) { chained = r.ExpectStatus(ft, 201) }), 0, 0)
	if chained.Status != 201 {
		t.Error("ExpectStatus must return the response so calls can be chained")
	}
	expect(t, run(func(ft *fakeT) { r.ExpectStatus(ft, 200) }), 1, 0)
}

func TestExpectHeader(t *testing.T) {
	r := Response{Header: http.Header{"Location": {"/tasks/1"}}}
	expect(t, run(func(ft *fakeT) { r.ExpectHeader(ft, "Location", "/tasks/1") }), 0, 0)
	expect(t, run(func(ft *fakeT) { r.ExpectHeader(ft, "location", "/tasks/1") }), 0, 0) // keys are case-insensitive
	expect(t, run(func(ft *fakeT) { r.ExpectHeader(ft, "Location", "/tasks/2") }), 1, 0)
	expect(t, run(func(ft *fakeT) { r.ExpectHeader(ft, "ETag", `"abc"`) }), 1, 0)
}

func TestExpectError(t *testing.T) {
	jsonCT := http.Header{"Content-Type": {"application/json"}}
	envelope := []byte(`{"error":{"code":"validation_failed","message":"invalid task"}}`)
	tests := []struct {
		name string
		r    Response
		want int // number of reported problems
	}{
		{"matches", Response{Status: 422, Header: jsonCT, Body: envelope}, 0},
		{"charset parameter is fine", Response{Status: 422, Header: http.Header{"Content-Type": {"application/json; charset=utf-8"}}, Body: envelope}, 0},
		{"wrong status", Response{Status: 400, Header: jsonCT, Body: envelope}, 1},
		{"wrong code", Response{Status: 422, Header: jsonCT, Body: []byte(`{"error":{"code":"bad_request"}}`)}, 1},
		{"not the envelope", Response{Status: 422, Header: jsonCT, Body: []byte(`{"detail":"nope"}`)}, 1},
		{"wrong content type", Response{Status: 422, Header: http.Header{"Content-Type": {"text/plain"}}, Body: envelope}, 1},
		{"body is not JSON", Response{Status: 422, Header: jsonCT, Body: []byte("oops")}, 1},
		{"everything wrong", Response{Status: 500, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte(`{"error":{"code":"internal"}}`)}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect(t, run(func(ft *fakeT) { tt.r.ExpectError(ft, 422, "validation_failed") }), tt.want, 0)
		})
	}
}
