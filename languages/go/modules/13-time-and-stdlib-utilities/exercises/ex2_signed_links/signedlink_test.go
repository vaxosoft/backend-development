package signedlink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

var (
	key     = []byte("test-key")
	expires = time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC)
	before  = expires.Add(-time.Minute)
)

func newSigner(t *testing.T, key []byte) *Signer {
	t.Helper()
	s, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestNewRejectsEmptyKey(t *testing.T) {
	if _, err := New(nil); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("New(nil) error = %v, want ErrEmptyKey", err)
	}
}

func TestRoundTrip(t *testing.T) {
	s := newSigner(t, key)
	token := s.Sign(42, expires)
	id, err := s.Verify(token, before)
	if err != nil || id != 42 {
		t.Fatalf("Verify = %d, %v; want 42, nil", id, err)
	}
	if token != s.Sign(42, expires) {
		t.Fatal("Sign is not deterministic")
	}
}

func TestTokenFormatIsPinned(t *testing.T) {
	token := newSigner(t, key).Sign(42, expires)
	m := hmac.New(sha256.New, key)
	m.Write([]byte("42.1893499200"))
	want := "42.1893499200." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
	if token != want {
		t.Fatalf("token = %q, want %q", token, want)
	}
}

func TestSubSecondPartIgnored(t *testing.T) {
	s := newSigner(t, key)
	if s.Sign(1, expires) != s.Sign(1, expires.Add(900*time.Millisecond)) {
		t.Fatal("expiry must be truncated to whole seconds")
	}
}

func TestExpiry(t *testing.T) {
	s := newSigner(t, key)
	token := s.Sign(7, expires)
	tests := []struct {
		name string
		now  time.Time
		want error
	}{
		{"just before", expires.Add(-time.Second), nil},
		{"exactly at expiry", expires, ErrExpired},
		{"after", expires.Add(time.Hour), ErrExpired},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := s.Verify(token, tc.now); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTamperingIsDetected(t *testing.T) {
	s := newSigner(t, key)
	token := s.Sign(42, expires)
	parts := strings.Split(token, ".")
	tests := map[string]string{
		"other task":     "43." + parts[1] + "." + parts[2],
		"extended":       parts[0] + ".9999999999." + parts[2],
		"flipped sig":    parts[0] + "." + parts[1] + "." + flip(parts[2]),
		"other key":      newSigner(t, []byte("other")).Sign(42, expires),
		"empty sig":      parts[0] + "." + parts[1] + ".",
		"truncated sig":  token[:len(token)-4],
		"signature only": "." + parts[2],
	}
	for name, tok := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := s.Verify(tok, before)
			if !errors.Is(err, ErrBadSignature) && !errors.Is(err, ErrMalformed) {
				t.Fatalf("Verify accepted or misclassified a forged token: %v", err)
			}
		})
	}
	if _, err := s.Verify(tests["extended"], before); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("extended expiry: error = %v, want ErrBadSignature", err)
	}
}

func TestForgedExpiredTokenReportsBadSignature(t *testing.T) {
	s := newSigner(t, key)
	parts := strings.Split(s.Sign(42, expires), ".")
	forged := parts[0] + ".1000000000." + parts[2]
	if _, err := s.Verify(forged, before); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("error = %v, want ErrBadSignature (signature is checked before expiry)", err)
	}
}

func TestMalformed(t *testing.T) {
	s := newSigner(t, key)
	for _, tok := range []string{"", "abc", "1.2", "1.2.3.4", "x.1.AAAA", "1.y.AAAA", "1.2.!!!!"} {
		t.Run(tok, func(t *testing.T) {
			if _, err := s.Verify(tok, before); !errors.Is(err, ErrMalformed) {
				t.Fatalf("Verify(%q) error = %v, want ErrMalformed", tok, err)
			}
		})
	}
}

// flip changes the first character of a base64url string to a different valid one.
func flip(s string) string {
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}
