// Command server runs the P3 file service.
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"webdev/projects/P3-file-service/starter"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	dir := os.Getenv("FILES_DIR")
	if dir == "" {
		dir = "data"
	}
	var maxSize int64
	if v := os.Getenv("MAX_UPLOAD_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			log.Fatalf("MAX_UPLOAD_BYTES must be a positive integer, got %q", v)
		}
		maxSize = n
	}

	h, err := starter.New(starter.Config{Dir: dir, MaxSize: maxSize})
	if err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	// No ReadTimeout/WriteTimeout covering the whole exchange: large transfers
	// on slow links would be cut off. Header and idle timeouts still apply.
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Printf("listening on %s (dir %s)", ln.Addr(), dir)
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		log.Fatal(err)
	}
	log.Print("shut down cleanly")
}
