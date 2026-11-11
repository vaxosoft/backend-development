package getjson

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

// fakeClient answers every request with the given status and body, without any network.
func fakeClient(status int, body string) (*http.Client, *trackedBody, *http.Request) {
	tb := &trackedBody{Reader: strings.NewReader(body)}
	var seen http.Request
	c := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = *r
		return &http.Response{StatusCode: status, Body: tb, Header: http.Header{}, Request: r}, nil
	})}
	return c, tb, &seen
}

func TestGetJSONDecodesAndSetsAccept(t *testing.T) {
	c, body, seen := fakeClient(200, `{"id":7,"title":"x"}`)
	var got struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	if err := GetJSON(context.Background(), c, "http://example.test/tasks/7", &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Title != "x" {
		t.Fatalf("got %+v", got)
	}
	if a := seen.Header.Get("Accept"); a != "application/json" {
		t.Errorf("Accept = %q", a)
	}
	if !body.closed {
		t.Error("response body was not closed")
	}
}

func TestGetJSONNon2xxIsStatusError(t *testing.T) {
	long := strings.Repeat("e", 5000)
	for _, code := range []int{301, 404, 500} {
		c, body, _ := fakeClient(code, long)
		var out map[string]any
		err := GetJSON(context.Background(), c, "http://example.test/", &out)
		var se *StatusError
		if !errors.As(err, &se) {
			t.Fatalf("status %d: err = %v, want *StatusError", code, err)
		}
		if se.Code != code {
			t.Errorf("Code = %d, want %d", se.Code, code)
		}
		if len(se.Body) != 1024 {
			t.Errorf("Body length = %d, want 1024", len(se.Body))
		}
		if !body.closed {
			t.Errorf("status %d: body not closed", code)
		}
	}
}

func TestGetJSONBadJSONIsNotStatusError(t *testing.T) {
	c, body, _ := fakeClient(200, `{not json`)
	var out map[string]any
	err := GetJSON(context.Background(), c, "http://example.test/", &out)
	if err == nil {
		t.Fatal("want decode error")
	}
	var se *StatusError
	if errors.As(err, &se) {
		t.Error("decode failure must not be a StatusError")
	}
	if !body.closed {
		t.Error("body not closed after decode failure")
	}
}

func TestGetJSONTransportErrorIsWrapped(t *testing.T) {
	boom := errors.New("boom")
	c := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, boom })}
	err := GetJSON(context.Background(), c, "http://example.test/", new(any))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap boom", err)
	}
}

func TestGetJSONHonoursContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := GetJSON(ctx, srv.Client(), srv.URL, new(any))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
