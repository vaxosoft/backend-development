// Package tokens is exercise 2: issue and verify HS256 JWTs without trusting the token's own header.
//
// Run the tests: go test ./modules/11-auth-and-security-basics/exercises/ex2_jwt_tokens/
package tokens

import (
	"errors"
	"time"
)

var (
	// ErrInvalid covers every reason a token must not be trusted (bad signature, wrong algorithm,
	// wrong issuer, missing expiry, malformed, ...).
	ErrInvalid = errors.New("invalid token")
	// ErrExpired means the token was well-formed and correctly signed but is past its expiry.
	ErrExpired = errors.New("token expired")
)

// Claims is what the application needs to know about the caller.
type Claims struct {
	UserID int
	Role   string
}

// Service issues and verifies tokens. Now is injectable so tests control time; nil means time.Now.
type Service struct {
	Secret []byte
	Issuer string
	TTL    time.Duration
	Now    func() time.Time
}

// Issue signs a token for c: HS256, subject = decimal user id, a "role" claim, the service issuer,
// issued-at = now and expiry = now + TTL. It fails if the secret is shorter than 32 bytes.
func (s Service) Issue(c Claims) (string, error) {
	panic("TODO")
}

// Verify checks a token and returns its claims. Only HS256 is accepted, and the issuer and an
// expiry must be present and valid. Expired tokens yield ErrExpired, everything else ErrInvalid
// (use errors.Is on the result).
func (s Service) Verify(token string) (Claims, error) {
	panic("TODO")
}
