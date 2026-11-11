// Package secrets is exercise 1: password hashing and random tokens done safely.
//
// Run the tests: go test ./modules/11-auth-and-security-basics/exercises/ex1_password_and_tokens/
package secrets

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

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

func (h Hasher) cost() int {
	if h.Cost == 0 {
		return bcrypt.DefaultCost
	}
	return h.Cost
}

// Hash returns the bcrypt hash of password, enforcing the length rules above.
func (h Hasher) Hash(password string) (string, error) {
	switch {
	case len(password) < 8:
		return "", ErrWeakPassword
	case len(password) > 72:
		return "", ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost())
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// Verify returns nil when password matches hash, ErrMismatch when it does not, and another error for a malformed hash.
func (h Hasher) Verify(hash, password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return ErrMismatch
	default:
		return fmt.Errorf("verify password: %w", err)
	}
}

// NeedsRehash reports whether hash was made with a lower cost than h.Cost (or is invalid).
func (h Hasher) NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	return err != nil || cost < h.cost()
}

// NewToken returns n random bytes from crypto/rand as 2n hex characters.
func NewToken(n int) (string, error) {
	if n <= 0 {
		return "", errors.New("token size must be positive")
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashToken returns the hex-encoded SHA-256 of token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TokenEqual compares two tokens in constant time.
func TokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
