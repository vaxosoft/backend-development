// Response writing: headers vs body order, negotiation, cookies, CORS, streaming, static files.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed static
var staticFS embed.FS

func newMux() http.Handler {
	mux := http.NewServeMux()

	// Content negotiation: a very small Accept check.
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept")
		if strings.Contains(r.Header.Get("Accept"), "text/plain") {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, "status: ok")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Headers must be set before the first Write or WriteHeader.
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/tasks/1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, `{"id":1}`)
		w.Header().Set("X-Too-Late", "ignored") // already sent: has no effect
	})

	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "abc123",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   3600,
		})
		fmt.Fprintln(w, "logged in")
	})
	mux.HandleFunc("GET /whoami", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if errors.Is(err, http.ErrNoCookie) {
			http.Error(w, "no session", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "session=%s\n", c.Value)
	})

	// Streaming: flush each chunk; stop when the client goes away.
	mux.HandleFunc("GET /stream", func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for i := 1; i <= 3; i++ {
			if r.Context().Err() != nil {
				return
			}
			fmt.Fprintf(w, "chunk %d\n", i)
			if err := rc.Flush(); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	// Static files from an embedded directory, mounted under /static/.
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // programmer error: the embed pattern is wrong
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(sub)))

	return cors("http://localhost:3000")(mux)
}

// cors allows one origin and answers preflight requests itself.
func cors(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") == origin {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
					h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
					h.Set("Access-Control-Max-Age", "600")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func main() {
	log.SetFlags(0)
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           newMux(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
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
