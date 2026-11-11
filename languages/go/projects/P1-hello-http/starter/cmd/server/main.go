// Command server runs the P1 hello-http service.
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webdev/projects/P1-hello-http/starter"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", ln.Addr())

	h := starter.New(starter.Config{LogSink: os.Stdout})
	if err := starter.Run(ctx, ln, h, 10*time.Second); err != nil {
		log.Fatal(err)
	}
	log.Print("shut down cleanly")
}
