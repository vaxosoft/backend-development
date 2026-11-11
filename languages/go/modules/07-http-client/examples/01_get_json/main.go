// Example 01: one shared client, a JSON round trip, and non-2xx handling.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
)

type Task struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// StatusError reports a non-2xx response: the client does not treat it as an error by itself.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("unexpected status %d", e.Code) }

// One client for the whole program so connections are pooled.
var client = &http.Client{Timeout: 5 * time.Second}

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return &StatusError{Code: resp.StatusCode, Body: string(body)}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/1" {
			http.Error(w, "no such task", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1,"title":"Write docs","status":"todo"}`)
	}))
	defer srv.Close()

	ctx := context.Background()

	var t Task
	if err := getJSON(ctx, srv.URL+"/tasks/1", &t); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("got %+v\n", t)

	err := getJSON(ctx, srv.URL+"/tasks/99", &t)
	var se *StatusError
	if errors.As(err, &se) {
		fmt.Printf("status %d, body %q\n", se.Code, se.Body)
	}
}
