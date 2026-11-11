// Example 03: connection reuse depends on draining and closing the body.
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
)

func main() {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, strings.Repeat("x", 1<<20))
	}))
	srv.Config.ConnState = func(_ net.Conn, s http.ConnState) {
		if s == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	run := func(name string, consume func(*http.Response)) {
		conns.Store(0)
		tr := &http.Transport{}
		defer tr.CloseIdleConnections()
		c := &http.Client{Transport: tr}
		for range 5 {
			resp, err := c.Get(srv.URL)
			if err != nil {
				panic(err)
			}
			consume(resp)
		}
		fmt.Printf("%-22s -> %d connection(s) for 5 requests\n", name, conns.Load())
	}

	run("drain and close", func(r *http.Response) {
		io.Copy(io.Discard, r.Body)
		r.Body.Close()
	})
	run("close without reading", func(r *http.Response) {
		r.Body.Close()
	})
}
