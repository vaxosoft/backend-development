// Package runtimeconfig is exercise 2: production settings from flags and environment.
//
// Run the tests: go test ./modules/14-packaging-and-running/exercises/ex2_runtime_config/
package runtimeconfig

import (
	"fmt"
	"io"
	"time"
)

// Config is the validated runtime configuration.
type Config struct {
	Port            int
	LogLevel        string
	Env             string
	ShutdownTimeout time.Duration
}

// ListenAddr is the address to pass to net.Listen, for example ":8080".
func (c Config) ListenAddr() string { return "" } // TODO

// FieldError reports one invalid setting; Field is the flag name (port, log-level, env, shutdown-timeout).
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return "invalid " + e.Field + ": " + e.Reason }

// Parse builds a Config. Use flag.NewFlagSet with flag.ContinueOnError and write usage output to out.
//
// Sources, lowest to highest precedence: defaults, environment, flags.
//
//	flag              env                default  rule
//	-port             PORT               8080     1-65535
//	-log-level        LOG_LEVEL          info     debug, info, warn, error
//	-env              ENV                dev      dev, prod
//	-shutdown-timeout SHUTDOWN_TIMEOUT   10s      > 0 and <= 5m
//
// Additional rules:
//   - debug logging is not allowed when env is prod (field log-level)
//   - unparseable PORT or SHUTDOWN_TIMEOUT values are a *FieldError for that field
//   - all validation problems are reported together with errors.Join, each one a *FieldError
//   - -h returns flag.ErrHelp (usage text goes to out); a malformed flag or leftover positional
//     arguments return an error; on any error the returned Config is the zero value.
func Parse(args []string, getenv func(string) string, out io.Writer) (Config, error) {
	return Config{}, fmt.Errorf("TODO") // TODO
}
