// Package settings is exercise 3: report every configuration problem at once, and never leak secrets.
package settings

import (
	"log/slog"
	"time"
)

// Secret is a string that must never show up in logs, JSON or fmt output.
//
// Required behaviour for every output path (fmt verbs %v %+v %s %q %#v, json.Marshal, slog text and
// JSON handlers): a non-empty secret prints as [redacted], an empty one prints as an empty string
// (so an operator can tell "unset" from "set"). Only Reveal returns the real value.
type Secret string

func (s Secret) String() string { return string(s) }

// Reveal returns the real value; call it only where the secret is actually needed.
func (s Secret) Reveal() string { return string(s) }

func (s Secret) LogValue() slog.Value { return slog.StringValue(string(s)) }

// Settings is the validated runtime configuration of the service.
type Settings struct {
	Addr          string        `json:"addr"`
	Env           string        `json:"env"`
	APIToken      Secret        `json:"api_token"`
	UpstreamURL   string        `json:"upstream_url"`
	Workers       int           `json:"workers"`
	ShutdownGrace time.Duration `json:"shutdown_grace"`
}

// FieldError is one invalid field. Field is the JSON name; Problem must never contain a secret value.
type FieldError struct {
	Field   string
	Problem string
}

func (e FieldError) Error() string { return e.Field + ": " + e.Problem }

// Validate checks all fields and reports ALL problems, not just the first, as an
// errors.Join of FieldError values in this field order. It returns nil when everything is valid.
//
//	addr            host:port (net.SplitHostPort) with a port in 1..65535
//	env             one of dev, staging, prod
//	api_token       at least 16 characters
//	upstream_url    optional; when set an absolute http or https URL with a host
//	workers         1..64
//	shutdown_grace  greater than 0 and at most 5 minutes
func (s Settings) Validate() error {
	panic("TODO")
}

// Problems returns every FieldError contained in err in order, looking through fmt.Errorf("%w"),
// errors.Join and nested joins. It returns nil for a nil error or when there are none.
// (errors.As only finds the first match.)
func Problems(err error) []FieldError {
	panic("TODO")
}
