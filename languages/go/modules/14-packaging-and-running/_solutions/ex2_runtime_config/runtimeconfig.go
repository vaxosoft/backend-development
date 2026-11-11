// Package runtimeconfig is exercise 2: production settings from flags and environment.
package runtimeconfig

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"time"
)

// Config is the validated runtime configuration.
type Config struct {
	Port            int
	LogLevel        string
	Env             string
	ShutdownTimeout time.Duration
}

// ListenAddr is the address to pass to net.Listen.
func (c Config) ListenAddr() string { return fmt.Sprintf(":%d", c.Port) }

// FieldError reports one invalid setting.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return "invalid " + e.Field + ": " + e.Reason }

// Parse applies defaults < environment < flags and validates the result.
func Parse(args []string, getenv func(string) string, out io.Writer) (Config, error) {
	var errs []error
	c := Config{Port: 8080, LogLevel: "info", Env: "dev", ShutdownTimeout: 10 * time.Second}

	if v := getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err != nil {
			errs = append(errs, &FieldError{"port", "PORT is not a number"})
		} else {
			c.Port = p
		}
	}
	if v := getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := getenv("ENV"); v != "" {
		c.Env = v
	}
	if v := getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err != nil {
			errs = append(errs, &FieldError{"shutdown-timeout", "SHUTDOWN_TIMEOUT is not a duration"})
		} else {
			c.ShutdownTimeout = d
		}
	}

	fs := flag.NewFlagSet("service", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.IntVar(&c.Port, "port", c.Port, "listen port (env PORT)")
	fs.StringVar(&c.LogLevel, "log-level", c.LogLevel, "debug, info, warn or error (env LOG_LEVEL)")
	fs.StringVar(&c.Env, "env", c.Env, "dev or prod (env ENV)")
	fs.DurationVar(&c.ShutdownTimeout, "shutdown-timeout", c.ShutdownTimeout, "drain time (env SHUTDOWN_TIMEOUT)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() > 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, &FieldError{"port", "must be 1-65535"})
	}
	if !slices.Contains([]string{"debug", "info", "warn", "error"}, c.LogLevel) {
		errs = append(errs, &FieldError{"log-level", "must be debug, info, warn or error"})
	}
	if !slices.Contains([]string{"dev", "prod"}, c.Env) {
		errs = append(errs, &FieldError{"env", "must be dev or prod"})
	}
	if c.Env == "prod" && c.LogLevel == "debug" {
		errs = append(errs, &FieldError{"log-level", "debug is not allowed in prod"})
	}
	if c.ShutdownTimeout <= 0 || c.ShutdownTimeout > 5*time.Minute {
		errs = append(errs, &FieldError{"shutdown-timeout", "must be in (0, 5m]"})
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return c, nil
}
