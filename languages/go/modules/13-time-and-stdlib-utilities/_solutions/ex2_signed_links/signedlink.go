// Package signedlink is exercise 2: expiring, tamper-proof links built from HMAC-SHA256.
package signedlink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrEmptyKey     = errors.New("signedlink: empty key")
	ErrMalformed    = errors.New("signedlink: malformed token")
	ErrBadSignature = errors.New("signedlink: bad signature")
	ErrExpired      = errors.New("signedlink: token expired")
)

// Signer creates and verifies tokens of the form "<taskID>.<expiresUnix>.<signature>".
type Signer struct{ key []byte }

// New returns a Signer for key.
func New(key []byte) (*Signer, error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	}
	return &Signer{key: append([]byte(nil), key...)}, nil
}

func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// Sign returns a token granting access to taskID until expires (whole seconds).
func (s *Signer) Sign(taskID int, expires time.Time) string {
	payload := strconv.Itoa(taskID) + "." + strconv.FormatInt(expires.Unix(), 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.mac(payload))
}

// Verify checks structure, then signature, then expiry, and returns the task id.
func (s *Signer) Verify(token string, now time.Time) (int, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, ErrMalformed
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, ErrMalformed
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, ErrMalformed
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return 0, ErrMalformed
	}
	if !hmac.Equal(sig, s.mac(parts[0]+"."+parts[1])) {
		return 0, ErrBadSignature
	}
	if !now.Before(time.Unix(exp, 0)) {
		return 0, ErrExpired
	}
	return id, nil
}
