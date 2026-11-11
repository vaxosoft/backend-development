// Package routes practises ServeMux patterns, path values and query parameters.
package routes

import "net/http"

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
	// TODO: register the routes above.
	return mux
}
