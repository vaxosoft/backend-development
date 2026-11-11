// Example 02: signing and verifying HS256 JWTs, and the checks that make verification safe.
//
//	go run ./modules/11-auth-and-security-basics/examples/02_jwt
package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secret = []byte("demo-secret-at-least-32-bytes-long!")

type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

func sign(method jwt.SigningMethod, key any, c Claims) string {
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		panic(err)
	}
	return s
}

// parse accepts only HS256, requires exp and checks the issuer; now is injectable for tests.
func parse(token string, now time.Time) (*Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer("tasktracker"),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func main() {
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	claims := Claims{Role: "member", RegisteredClaims: jwt.RegisteredClaims{
		Issuer:    "tasktracker",
		Subject:   "42",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	token := sign(jwt.SigningMethodHS256, secret, claims)

	parts := strings.Split(token, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	fmt.Println("parts:", len(parts))
	fmt.Println("payload is only encoded, not secret:", string(payload))

	c, err := parse(token, now.Add(time.Minute))
	fmt.Println("valid:", c.Subject, c.Role, err)

	_, err = parse(token, now.Add(2*time.Hour))
	fmt.Println("expired:", errors.Is(err, jwt.ErrTokenExpired))

	forged := parts[0] + "." + base64.RawURLEncoding.EncodeToString([]byte(
		`{"role":"admin","iss":"tasktracker","sub":"42","exp":1893488400}`)) + "." + parts[2]
	_, err = parse(forged, now)
	fmt.Println("tampered payload:", errors.Is(err, jwt.ErrTokenSignatureInvalid))

	none := sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, claims)
	_, err = parse(none, now)
	fmt.Println("alg none rejected:", errors.Is(err, jwt.ErrTokenSignatureInvalid))

	hs512 := sign(jwt.SigningMethodHS512, secret, claims)
	_, err = parse(hs512, now)
	fmt.Println("other alg rejected:", errors.Is(err, jwt.ErrTokenSignatureInvalid))

	other := claims
	other.Issuer = "someone-else"
	_, err = parse(sign(jwt.SigningMethodHS256, secret, other), now)
	fmt.Println("wrong issuer:", errors.Is(err, jwt.ErrTokenInvalidIssuer))

	noExp := claims
	noExp.ExpiresAt = nil
	_, err = parse(sign(jwt.SigningMethodHS256, secret, noExp), now)
	fmt.Println("missing exp:", errors.Is(err, jwt.ErrTokenRequiredClaimMissing))
}
