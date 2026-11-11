// Package tokens is exercise 2: issue and verify HS256 JWTs without trusting the token's own header.
//
// Run the tests: go test ./modules/11-auth-and-security-basics/exercises/ex2_jwt_tokens/
package tokens

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

type wireClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Issue signs a token for c: HS256, subject = decimal user id, a "role" claim, the service issuer,
// issued-at = now and expiry = now + TTL. It fails if the secret is shorter than 32 bytes.
func (s Service) Issue(c Claims) (string, error) {
	if len(s.Secret) < 32 {
		return "", errors.New("secret must be at least 32 bytes")
	}
	now := s.now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, wireClaims{
		Role: c.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.Issuer,
			Subject:   strconv.Itoa(c.UserID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.TTL)),
		},
	})
	signed, err := token.SignedString(s.Secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Verify checks a token and returns its claims. Only HS256 is accepted, and the issuer and an
// expiry must be present and valid. Expired tokens yield ErrExpired, everything else ErrInvalid
// (use errors.Is on the result).
func (s Service) Verify(token string) (Claims, error) {
	var wc wireClaims
	_, err := jwt.ParseWithClaims(token, &wc, func(*jwt.Token) (any, error) { return s.Secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(s.Issuer),
		jwt.WithTimeFunc(s.now),
	)
	if errors.Is(err, jwt.ErrTokenExpired) {
		return Claims{}, ErrExpired
	}
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	id, err := strconv.Atoi(wc.Subject)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: bad subject", ErrInvalid)
	}
	return Claims{UserID: id, Role: wc.Role}, nil
}
