// Command fakeupstream is a tiny upstream for trying the aggregator by hand.
//
//	GET /items/{id} -> {"id":N,"name":"item-N"}
//	id 13           -> always 500
//	id 404          -> 404
//	id 99           -> answers after 3 s
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:9000"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		switch id {
		case 13:
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		case 404:
			http.NotFound(w, r)
			return
		case 99:
			select {
			case <-time.After(3 * time.Second):
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"name":"item-%d"}`+"\n", id, id)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("fake upstream on %s", addr)
	log.Fatal(srv.ListenAndServe())
}
