// Package envparse is exercise 1: typed access to environment variables through an injectable lookup.
package envparse

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

// Lookup has the signature of os.LookupEnv; tests pass a map-backed fake.
type Lookup func(key string) (string, bool)

// ParseError describes a variable whose value could not be used. Err is the underlying cause
// (for example *strconv.NumError), reachable with errors.Is / errors.As.
type ParseError struct {
	Key   string
	Value string
	Err   error
}

func (e *ParseError) Error() string { return "invalid " + e.Key + "=" + e.Value + ": " + e.Err.Error() }
func (e *ParseError) Unwrap() error { return e.Err }

// raw returns the trimmed value and whether there is one.
func raw(l Lookup, key string) (string, bool) {
	v, ok := l(key)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

// For all functions below:
//   - a variable that is unset, or set to "" (after trimming spaces), yields def and no error;
//   - surrounding spaces of a set value are trimmed before parsing;
//   - an unusable value returns the zero value and a *ParseError carrying Key and the trimmed Value.

// String returns the trimmed value of key or def.
func String(l Lookup, key, def string) string {
	if v, ok := raw(l, key); ok {
		return v
	}
	return def
}

// Int parses a base-10 integer.
func Int(l Lookup, key string, def int) (int, error) {
	v, ok := raw(l, key)
	if !ok {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, &ParseError{Key: key, Value: v, Err: err}
	}
	return n, nil
}

// Bool accepts what strconv.ParseBool accepts (1, t, true, 0, f, false, ...).
func Bool(l Lookup, key string, def bool) (bool, error) {
	v, ok := raw(l, key)
	if !ok {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, &ParseError{Key: key, Value: v, Err: err}
	}
	return b, nil
}

// Duration parses values such as "500ms" or "2m"; negative durations are rejected.
func Duration(l Lookup, key string, def time.Duration) (time.Duration, error) {
	v, ok := raw(l, key)
	if !ok {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err == nil && d < 0 {
		err = errors.New("must not be negative")
	}
	if err != nil {
		return 0, &ParseError{Key: key, Value: v, Err: err}
	}
	return d, nil
}

// List splits a comma-separated value, trims every item and drops empty items.
// An unset or blank variable yields def.
func List(l Lookup, key string, def []string) []string {
	v, ok := raw(l, key)
	if !ok {
		return def
	}
	var out []string
	for _, item := range strings.Split(v, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
