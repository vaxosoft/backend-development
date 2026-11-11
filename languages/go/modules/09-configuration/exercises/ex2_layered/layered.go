// Package layered is exercise 2: configuration from defaults < JSON file < environment < flags.
package layered

import (
	"errors"
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
	panic("TODO")
}
