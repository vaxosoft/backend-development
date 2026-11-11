// Package signedlink is exercise 2: expiring, tamper-proof links built from HMAC-SHA256.
//
// Run the tests: go test ./modules/13-time-and-stdlib-utilities/exercises/ex2_signed_links/
package signedlink

import (
	"errors"
	"time"
)

var (
	ErrEmptyKey     = errors.New("signedlink: empty key")
	ErrMalformed    = errors.New("signedlink: malformed token")
	ErrBadSignature = errors.New("signedlink: bad signature")
	ErrExpired      = errors.New("signedlink: token expired")
)

// Signer creates and verifies tokens of the form "<taskID>.<expiresUnix>.<signature>"
// where signature is base64.RawURLEncoding of HMAC-SHA256(key, "<taskID>.<expiresUnix>").
type Signer struct{ key []byte }

// New returns a Signer for key, or ErrEmptyKey when key is empty.
func New(key []byte) (*Signer, error) {
	return nil, errors.New("TODO")
}

// Sign returns a token granting access to taskID until expires (whole Unix seconds).
func (s *Signer) Sign(taskID int, expires time.Time) string {
	return "" // TODO
}

// Verify returns the task id of a valid token. Check in this order:
//  1. structure and encoding of every part: ErrMalformed
//  2. signature, compared in constant time: ErrBadSignature
//  3. expiry: ErrExpired once now is at or after the expiry instant
//
// Checking the signature before the expiry means a forged token never learns whether it was "expired".
func (s *Signer) Verify(token string, now time.Time) (int, error) {
	return 0, errors.New("TODO")
}
