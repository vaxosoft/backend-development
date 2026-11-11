// Runtime flags and production settings: defaults < environment < flags, validated before anything starts.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"runtime/debug"
	"slices"
	"time"
)

type Config struct {
	Addr            string
	Env             string
	ShutdownTimeout time.Duration
	MemLimitMiB     int64
}

func envOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

// Parse takes its inputs as arguments (not os.Args/os.Getenv), so it is trivial to test.
func Parse(args []string, getenv func(string) string, out io.Writer) (Config, error) {
	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(out)
	var c Config
	fs.StringVar(&c.Addr, "addr", envOr(getenv, "ADDR", "127.0.0.1:8080"), "listen address (env ADDR)")
	fs.StringVar(&c.Env, "env", envOr(getenv, "APP_ENV", "dev"), "dev or prod (env APP_ENV)")
	fs.DurationVar(&c.ShutdownTimeout, "shutdown-timeout", 10*time.Second, "time allowed to drain in-flight requests")
	fs.Int64Var(&c.MemLimitMiB, "mem-limit-mib", 0, "soft memory limit for the Go runtime in MiB (0 = off)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if !slices.Contains([]string{"dev", "prod"}, c.Env) {
		return Config{}, fmt.Errorf("-env must be dev or prod, got %q", c.Env)
	}
	if c.ShutdownTimeout <= 0 || c.MemLimitMiB < 0 {
		return Config{}, errors.New("-shutdown-timeout must be positive and -mem-limit-mib not negative")
	}
	return c, nil
}

func run(args []string, getenv func(string) string, out io.Writer) error {
	c, err := Parse(args, getenv, out)
	if err != nil {
		return err
	}
	if c.MemLimitMiB > 0 {
		// A soft limit makes the GC work harder near the limit; set it below the container limit.
		debug.SetMemoryLimit(c.MemLimitMiB << 20)
	}
	limit := "off"
	if l := debug.SetMemoryLimit(-1); l != math.MaxInt64 { // -1 only reads the current limit
		limit = fmt.Sprintf("%d MiB", l>>20)
	}
	fmt.Fprintf(out, "addr=%s env=%s shutdown=%s mem-limit=%s\n", c.Addr, c.Env, c.ShutdownTimeout, limit)
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}
