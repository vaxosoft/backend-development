// Package settings is exercise 3: report every configuration problem at once, and never leak secrets.
package settings

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"time"
)

// Secret is a string that must never show up in logs, JSON or fmt output.
//
// Required behaviour for every output path (fmt verbs %v %+v %s %q %#v, json.Marshal, slog text and
// JSON handlers): a non-empty secret prints as [redacted], an empty one prints as an empty string
// (so an operator can tell "unset" from "set"). Only Reveal returns the real value.
type Secret string

func (s Secret) String() string {
	if s == "" {
		return ""
	}
	return "[redacted]"
}

// GoString covers %#v.
func (s Secret) GoString() string { return strconv.Quote(s.String()) }

// MarshalText covers encoding/json (and any other text-based encoder).
func (s Secret) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Reveal returns the real value; call it only where the secret is actually needed.
func (s Secret) Reveal() string { return string(s) }

func (s Secret) LogValue() slog.Value { return slog.StringValue(s.String()) }

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
	var errs []error
	add := func(field, problem string) { errs = append(errs, FieldError{field, problem}) }

	if _, port, err := net.SplitHostPort(s.Addr); err != nil {
		add("addr", "must be host:port")
	} else if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		add("addr", "port must be 1-65535")
	}
	switch s.Env {
	case "dev", "staging", "prod":
	default:
		add("env", "must be dev, staging or prod")
	}
	if len(s.APIToken.Reveal()) < 16 {
		add("api_token", "must be at least 16 characters")
	}
	if s.UpstreamURL != "" {
		if u, err := url.Parse(s.UpstreamURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			add("upstream_url", "must be an absolute http or https URL")
		}
	}
	if s.Workers < 1 || s.Workers > 64 {
		add("workers", "must be 1-64")
	}
	if s.ShutdownGrace <= 0 || s.ShutdownGrace > 5*time.Minute {
		add("shutdown_grace", "must be greater than 0 and at most 5m")
	}
	return errors.Join(errs...)
}

// Problems returns every FieldError contained in err in order, looking through fmt.Errorf("%w"),
// errors.Join and nested joins. It returns nil for a nil error or when there are none.
// (errors.As only finds the first match.)
func Problems(err error) []FieldError {
	var out []FieldError
	var walk func(error)
	walk = func(e error) {
		switch e := e.(type) {
		case nil:
		case FieldError:
			out = append(out, e)
		case interface{ Unwrap() []error }:
			for _, c := range e.Unwrap() {
				walk(c)
			}
		case interface{ Unwrap() error }:
			walk(e.Unwrap())
		}
	}
	walk(err)
	return out
}
