package runtimeconfig

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"testing"
	"time"
)

func env(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

// fields lists the Field of every *FieldError inside err (including joined ones).
func fields(err error) []string {
	var out []string
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		if fe, ok := e.(*FieldError); ok {
			out = append(out, fe.Field)
			return
		}
		switch u := e.(type) {
		case interface{ Unwrap() []error }:
			for _, c := range u.Unwrap() {
				walk(c)
			}
		case interface{ Unwrap() error }:
			walk(u.Unwrap())
		}
	}
	walk(err)
	slices.Sort(out)
	return out
}

func TestDefaults(t *testing.T) {
	c, err := Parse(nil, env(), new(bytes.Buffer))
	want := Config{Port: 8080, LogLevel: "info", Env: "dev", ShutdownTimeout: 10 * time.Second}
	if err != nil || c != want {
		t.Fatalf("got %+v, %v; want %+v", c, err, want)
	}
	if got := c.ListenAddr(); got != ":8080" {
		t.Fatalf("ListenAddr = %q", got)
	}
}

func TestPrecedenceDefaultsEnvFlags(t *testing.T) {
	getenv := env("PORT", "9000", "LOG_LEVEL", "warn", "ENV", "prod", "SHUTDOWN_TIMEOUT", "30s")
	c, err := Parse(nil, getenv, new(bytes.Buffer))
	if err != nil {
		t.Fatal(err)
	}
	if c != (Config{9000, "warn", "prod", 30 * time.Second}) {
		t.Fatalf("env not applied: %+v", c)
	}
	c, err = Parse([]string{"-port", "7000", "-log-level=error", "-shutdown-timeout", "2s"}, getenv, new(bytes.Buffer))
	if err != nil {
		t.Fatal(err)
	}
	if c != (Config{7000, "error", "prod", 2 * time.Second}) {
		t.Fatalf("flags must override env: %+v", c)
	}
}

func TestAllProblemsReportedTogether(t *testing.T) {
	_, err := Parse([]string{"-port", "70000", "-log-level", "loud", "-env", "staging", "-shutdown-timeout", "0s"}, env(), new(bytes.Buffer))
	want := []string{"env", "log-level", "port", "shutdown-timeout"}
	if got := fields(err); !slices.Equal(got, want) {
		t.Fatalf("fields = %v, want %v (err: %v)", got, want, err)
	}
	var fe *FieldError
	if !errors.As(err, &fe) {
		t.Fatal("errors.As cannot find a *FieldError in the joined error")
	}
}

func TestPortBounds(t *testing.T) {
	for _, tc := range []struct {
		port string
		ok   bool
	}{{"0", false}, {"1", true}, {"65535", true}, {"65536", false}, {"-1", false}} {
		_, err := Parse([]string{"-port=" + tc.port}, env(), new(bytes.Buffer))
		if (err == nil) != tc.ok {
			t.Errorf("port %s: err = %v, want ok=%v", tc.port, err, tc.ok)
		}
	}
}

func TestBadEnvironmentValues(t *testing.T) {
	_, err := Parse(nil, env("PORT", "eighty"), new(bytes.Buffer))
	if got := fields(err); !slices.Equal(got, []string{"port"}) {
		t.Fatalf("PORT=eighty: fields = %v (err %v)", got, err)
	}
	_, err = Parse(nil, env("SHUTDOWN_TIMEOUT", "soon"), new(bytes.Buffer))
	if got := fields(err); !slices.Equal(got, []string{"shutdown-timeout"}) {
		t.Fatalf("SHUTDOWN_TIMEOUT=soon: fields = %v (err %v)", got, err)
	}
	// A valid flag replaces the broken environment value, but the broken value was still a mistake.
	if _, err := Parse([]string{"-port", "80"}, env("PORT", "eighty"), new(bytes.Buffer)); err == nil {
		t.Fatal("unparseable PORT must be reported even when a flag overrides it")
	}
}

func TestShutdownTimeoutLimit(t *testing.T) {
	if _, err := Parse([]string{"-shutdown-timeout", "5m"}, env(), new(bytes.Buffer)); err != nil {
		t.Fatalf("5m should be allowed: %v", err)
	}
	if _, err := Parse([]string{"-shutdown-timeout", "5m1s"}, env(), new(bytes.Buffer)); err == nil {
		t.Fatal("5m1s should be rejected")
	}
}

func TestProductionForbidsDebug(t *testing.T) {
	_, err := Parse([]string{"-env", "prod", "-log-level", "debug"}, env(), new(bytes.Buffer))
	if got := fields(err); !slices.Equal(got, []string{"log-level"}) {
		t.Fatalf("fields = %v (err %v)", got, err)
	}
	if _, err := Parse([]string{"-env", "dev", "-log-level", "debug"}, env(), new(bytes.Buffer)); err != nil {
		t.Fatalf("debug is fine in dev: %v", err)
	}
	if _, err := Parse([]string{"-log-level", "debug"}, env("ENV", "prod"), new(bytes.Buffer)); err == nil {
		t.Fatal("ENV=prod must also forbid -log-level debug")
	}
}

func TestHelpAndMalformedFlags(t *testing.T) {
	var out bytes.Buffer
	c, err := Parse([]string{"-h"}, env(), &out)
	if !errors.Is(err, flag.ErrHelp) || c != (Config{}) {
		t.Fatalf("-h: got %+v, %v", c, err)
	}
	if !bytes.Contains(out.Bytes(), []byte("-shutdown-timeout")) {
		t.Fatalf("usage should list the flags, got %q", out.String())
	}
	for _, args := range [][]string{{"-port", "abc"}, {"-nope"}, {"extra"}, {"-port"}} {
		out.Reset()
		if c, err := Parse(args, env(), &out); err == nil || c != (Config{}) {
			t.Errorf("Parse(%v) = %+v, %v; want zero Config and an error", args, c, err)
		}
	}
}

func TestParseDoesNotWriteToStdout(t *testing.T) {
	// All output must go through the writer passed in.
	var out bytes.Buffer
	_, _ = Parse([]string{"-bogus"}, env(), &out)
	if out.Len() == 0 {
		t.Fatal("expected the flag error and usage in the supplied writer")
	}
}
