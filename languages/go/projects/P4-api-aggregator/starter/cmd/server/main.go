// Command server runs the API aggregator.
//
//	UPSTREAM_URL=http://localhost:9000 ADDR=127.0.0.1:8080 go run ./projects/P4-api-aggregator/starter/cmd/server
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	aggregator "webdev/projects/P4-api-aggregator/starter"
)

func main() {
	cfg, err := aggregator.ConfigFromEnv(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           aggregator.New(cfg),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      cfg.OverallTimeout + 5*time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s, upstream %s", addr, cfg.UpstreamURL)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
