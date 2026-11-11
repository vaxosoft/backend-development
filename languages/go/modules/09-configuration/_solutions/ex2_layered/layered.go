// Package layered is exercise 2: configuration from defaults < JSON file < environment < flags.
package layered

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// ErrInvalid wraps every problem Load reports (bad file, bad value, bad flag, failed validation).
var ErrInvalid = errors.New("invalid configuration")

// Config is the effective configuration.
type Config struct {
	Addr     string
	LogLevel string
	Timeout  time.Duration
	Metrics  bool
}

// Lookup has the signature of os.LookupEnv.
type Lookup func(key string) (string, bool)

// Defaults is the lowest layer.
func Defaults() Config {
	return Config{Addr: "127.0.0.1:8080", LogLevel: "info", Timeout: 5 * time.Second, Metrics: true}
}

// fileConfig uses pointers so "absent" and "zero" can be told apart.
type fileConfig struct {
	Addr     *string `json:"addr"`
	LogLevel *string `json:"log_level"`
	Timeout  *string `json:"timeout"`
	Metrics  *bool   `json:"metrics"`
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Load builds the effective configuration. Later layers override earlier ones, field by field:
//
//  1. Defaults().
//  2. file: JSON with the optional keys "addr", "log_level", "timeout" (a duration string such as
//     "2s") and "metrics" (bool). nil or empty file = no file. A key that is absent leaves the lower
//     layer's value alone, even when the file sets "metrics": false for another run. Unknown keys and
//     malformed JSON are errors.
//  3. environment: ADDR, LOG_LEVEL, TIMEOUT, METRICS. Unset or empty variables are ignored.
//  4. flags (args, without the program name): -addr, -log-level, -timeout, -metrics. Only flags that
//     appear in args override; a flag left out must not reset a value coming from the file or environment,
//     and a flag given with a value equal to the default still counts. Unknown flags and positional
//     arguments are errors.
//
// After merging, validate: LogLevel is one of debug, info, warn, error; Timeout > 0; Addr is host:port
// (net.SplitHostPort). On any error return the zero Config and an error for which
// errors.Is(err, ErrInvalid) is true.
func Load(args []string, lookup Lookup, file []byte) (Config, error) {
	cfg := Defaults()

	if len(bytes.TrimSpace(file)) > 0 {
		var fc fileConfig
		dec := json.NewDecoder(bytes.NewReader(file))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&fc); err != nil {
			return Config{}, invalid("file: %v", err)
		}
		if fc.Addr != nil {
			cfg.Addr = *fc.Addr
		}
		if fc.LogLevel != nil {
			cfg.LogLevel = *fc.LogLevel
		}
		if fc.Timeout != nil {
			d, err := time.ParseDuration(*fc.Timeout)
			if err != nil {
				return Config{}, invalid("file timeout: %v", err)
			}
			cfg.Timeout = d
		}
		if fc.Metrics != nil {
			cfg.Metrics = *fc.Metrics
		}
	}

	get := func(key string) (string, bool) {
		v, ok := lookup(key)
		v = strings.TrimSpace(v)
		return v, ok && v != ""
	}
	if v, ok := get("ADDR"); ok {
		cfg.Addr = v
	}
	if v, ok := get("LOG_LEVEL"); ok {
		cfg.LogLevel = v
	}
	if v, ok := get("TIMEOUT"); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, invalid("TIMEOUT: %v", err)
		}
		cfg.Timeout = d
	}
	if v, ok := get("METRICS"); ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, invalid("METRICS: %v", err)
		}
		cfg.Metrics = b
	}

	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", cfg.Addr, "")
	level := fs.String("log-level", cfg.LogLevel, "")
	timeout := fs.Duration("timeout", cfg.Timeout, "")
	metrics := fs.Bool("metrics", cfg.Metrics, "")
	if err := fs.Parse(args); err != nil {
		return Config{}, invalid("flags: %v", err)
	}
	if fs.NArg() > 0 {
		return Config{}, invalid("unexpected argument %q", fs.Arg(0))
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			cfg.Addr = *addr
		case "log-level":
			cfg.LogLevel = *level
		case "timeout":
			cfg.Timeout = *timeout
		case "metrics":
			cfg.Metrics = *metrics
		}
	})

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, invalid("log level %q", cfg.LogLevel)
	}
	if cfg.Timeout <= 0 {
		return Config{}, invalid("timeout must be positive")
	}
	if _, _, err := net.SplitHostPort(cfg.Addr); err != nil {
		return Config{}, invalid("addr: %v", err)
	}
	return cfg, nil
}
