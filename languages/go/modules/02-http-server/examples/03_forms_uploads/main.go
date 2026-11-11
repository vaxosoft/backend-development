// Request parsing: URL-encoded forms, streamed multipart uploads, size limits.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const maxUpload = 1 << 20 // 1 MiB

func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /form", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		// PostForm holds the body only; r.Form would also merge the query string.
		fmt.Fprintf(w, "title=%q tags=%q\n", r.PostForm.Get("title"), r.PostForm["tag"])
	})

	mux.HandleFunc("POST /upload", upload)
	return mux
}

type uploaded struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// upload streams each part through a hash: memory use stays constant however
// large the file is, unlike r.ParseMultipartForm which buffers parts.
func upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "expected multipart/form-data", http.StatusBadRequest)
		return
	}
	var files []uploaded
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeReadError(w, err)
			return
		}
		if part.FileName() == "" { // an ordinary form field
			continue
		}
		h := sha256.New()
		n, err := io.Copy(h, part)
		if err != nil {
			writeReadError(w, err)
			return
		}
		files = append(files, uploaded{
			Name:   filepath.Base(part.FileName()), // never trust a client path
			Size:   n,
			SHA256: hex.EncodeToString(h.Sum(nil)),
		})
	}
	if len(files) == 0 {
		http.Error(w, `missing file part`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(files)
}

func writeReadError(w http.ResponseWriter, err error) {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		http.Error(w, fmt.Sprintf("upload exceeds %d bytes", tooBig.Limit), http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "malformed multipart body", http.StatusBadRequest)
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
		ReadTimeout:       30 * time.Second, // bounds a slow upload
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
