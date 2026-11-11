// Package envparse is exercise 1: typed access to environment variables through an injectable lookup.
package envparse

import (
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

// For all functions below:
//   - a variable that is unset, or set to "" (after trimming spaces), yields def and no error;
//   - surrounding spaces of a set value are trimmed before parsing;
//   - an unusable value returns the zero value and a *ParseError carrying Key and the trimmed Value.

// String returns the trimmed value of key or def.
func String(l Lookup, key, def string) string {
	panic("TODO")
}

// Int parses a base-10 integer.
func Int(l Lookup, key string, def int) (int, error) {
	panic("TODO")
}

// Bool accepts what strconv.ParseBool accepts (1, t, true, 0, f, false, ...).
func Bool(l Lookup, key string, def bool) (bool, error) {
	panic("TODO")
}

// Duration parses values such as "500ms" or "2m"; negative durations are rejected.
func Duration(l Lookup, key string, def time.Duration) (time.Duration, error) {
	panic("TODO")
}

// List splits a comma-separated value, trims every item and drops empty items.
// An unset or blank variable yields def.
func List(l Lookup, key string, def []string) []string {
	panic("TODO")
}
