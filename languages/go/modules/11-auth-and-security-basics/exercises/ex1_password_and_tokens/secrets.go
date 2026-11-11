// Package secrets is exercise 1: password hashing and random tokens done safely.
//
// Run the tests: go test ./modules/11-auth-and-security-basics/exercises/ex1_password_and_tokens/
package secrets

import "errors"

var (
	// ErrWeakPassword is returned for passwords shorter than 8 bytes.
	ErrWeakPassword = errors.New("password too short")
	// ErrPasswordTooLong is returned for passwords longer than 72 bytes, which bcrypt cannot use.
	ErrPasswordTooLong = errors.New("password too long")
	// ErrMismatch is returned by Verify when the password does not match the hash.
	ErrMismatch = errors.New("password does not match")
)

// Hasher hashes passwords with bcrypt. A zero Cost means bcrypt.DefaultCost.
type Hasher struct {
	Cost int
}

// Hash returns the bcrypt hash of password, enforcing the length rules above.
func (h Hasher) Hash(password string) (string, error) {
	panic("TODO")
}

// Verify returns nil when password matches hash, ErrMismatch (testable with errors.Is) when it does
// not, and a different, non-nil error when hash is not a valid bcrypt hash.
func (h Hasher) Verify(hash, password string) error {
	panic("TODO")
}

// NeedsRehash reports whether hash was made with a lower cost than h.Cost (or is not a valid hash),
// so the caller can re-hash the password after a successful login.
func (h Hasher) NeedsRehash(hash string) bool {
	panic("TODO")
}

// NewToken returns n random bytes from crypto/rand as 2n hex characters. n must be positive.
func NewToken(n int) (string, error) {
	panic("TODO")
}

// HashToken returns the hex-encoded SHA-256 of token, suitable for storing API keys and session ids.
func HashToken(token string) string {
	panic("TODO")
}

// TokenEqual compares two tokens in constant time.
func TokenEqual(a, b string) bool {
	panic("TODO")
}
