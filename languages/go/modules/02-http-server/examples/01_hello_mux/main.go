// Routing with the Go 1.22+ ServeMux: methods, wildcards, query parameters.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	// "{$}" matches only the root; plain "/" would match every unknown path.
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Task Tracker API")
	})

	mux.HandleFunc("GET /hello", hello)
	mux.HandleFunc("GET /hello/{name}", hello)

	// Query parameters come from r.URL.Query(); Get returns "" when absent.
	mux.HandleFunc("GET /search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		limit := 10
		if s := q.Get("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return // http.Error does not stop the handler
			}
			limit = n
		}
		fmt.Fprintf(w, "q=%q tags=%v limit=%d\n", q.Get("q"), q["tag"], limit)
	})

	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "task id must be a number", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "task %d\n", id)
	})

	// "{path...}" captures the rest of the URL, slashes included.
	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "file %q (%d segments)\n", r.PathValue("path"), len(strings.Split(r.PathValue("path"), "/")))
	})

	return mux
}

func hello(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name") // "" when the pattern has no {name}
	if name == "" {
		name = "world"
	}
	fmt.Fprintf(w, "Hello, %s!\n", name)
}

func main() {
	log.SetFlags(0)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	// A zero-value http.Server (or http.ListenAndServe) has no timeouts at all.
	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Printf("listening on %s", addr)

	select {
	case err := <-errc:
		log.Fatal(err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	log.Print("bye")
}
