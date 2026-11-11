// Package limiter is exercise 3: a per-key token-bucket rate limiter and the HTTP middleware around it.
//
// Run the tests: go test -race ./modules/11-auth-and-security-basics/exercises/ex3_rate_limiter/
package limiter

import (
	"net/http"
	"time"
)

// Limiter keeps one token bucket per key (for example a user id or API key).
// Each bucket holds up to burst tokens and refills at perSecond tokens per second.
// It must be safe for concurrent use.
type Limiter struct {
	burst     int
	perSecond float64
	now       func() time.Time
}

// New returns a Limiter. now is the clock (nil means time.Now); tests pass a fake one.
func New(burst int, perSecond float64, now func() time.Time) *Limiter {
	return &Limiter{burst: burst, perSecond: perSecond, now: now}
}

// Allow takes one token for key. When none is left it returns false and how long the caller must
// wait for the next token. A rejected call must not consume anything: asking again at the same
// instant reports the same retryAfter.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	return true, 0
}

// Middleware rejects requests whose key (from keyFn) is over its limit with 429, a Retry-After
// header in whole seconds (rounded up, at least 1) and the JSON error envelope
// {"error":{"code":"rate_limited","message":"..."}}. Other requests reach next unchanged.
func Middleware(l *Limiter, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return next
	}
}
