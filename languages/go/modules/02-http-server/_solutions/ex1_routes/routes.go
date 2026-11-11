// Package routes practises ServeMux patterns, path values and query parameters.
package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// NewMux returns the application's router. Register these routes:
//
//	GET /hello                  -> 200 "Hello, world!\n"
//	GET /hello/{name}           -> 200 "Hello, <name>!\n"
//	    ?shout=true upper-cases the whole greeting ("HELLO, ADA!\n")
//	    a name longer than 50 bytes -> 400 with body "name too long\n" (nothing else written)
//	GET /tasks/{id}             -> 200 "task <id>\n"; id must be a positive integer,
//	                               otherwise 400 with body "invalid id\n"
//	GET /files/{path...}        -> 200 "file: <path>\n" (path may contain slashes)
//
// Everything else is left to the mux: unknown paths give 404 and a wrong
// method on a known path gives 405 with an Allow header.
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", hello)
	mux.HandleFunc("GET /hello/{name}", hello)
	mux.HandleFunc("GET /tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil || id < 1 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "task %d\n", id)
	})
	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "file: %s\n", r.PathValue("path"))
	})
	return mux
}

func hello(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		name = "world"
	}
	if len(name) > 50 {
		http.Error(w, "name too long", http.StatusBadRequest)
		return
	}
	greeting := "Hello, " + name + "!\n"
	if r.URL.Query().Get("shout") == "true" {
		greeting = strings.ToUpper(greeting)
	}
	fmt.Fprint(w, greeting)
}
