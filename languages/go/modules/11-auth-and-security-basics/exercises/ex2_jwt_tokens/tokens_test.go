package tokens

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	secret = []byte("0123456789abcdef0123456789abcdef")
	t0     = time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
)

func newService(now *time.Time) Service {
	return Service{Secret: secret, Issuer: "tasktracker", TTL: time.Hour, Now: func() time.Time { return *now }}
}

func sign(t *testing.T, method jwt.SigningMethod, key any, c jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, c).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{"iss": "tasktracker", "sub": "7", "role": "admin", "exp": t0.Add(time.Hour).Unix()}
}

func TestRoundTrip(t *testing.T) {
	now := t0
	svc := newService(&now)
	token, err := svc.Issue(Claims{UserID: 7, Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != (Claims{UserID: 7, Role: "admin"}) {
		t.Fatalf("claims = %+v", got)
	}
}

func TestIssuedTokenShape(t *testing.T) {
	now := t0
	token, err := newService(&now).Issue(Claims{UserID: 7, Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	var mc jwt.MapClaims
	parsed, _, err := jwt.NewParser().ParseUnverified(token, &mc)
	if err != nil {
		t.Fatalf("not a JWT: %v", err)
	}
	if parsed.Method.Alg() != "HS256" {
		t.Errorf("alg = %s", parsed.Method.Alg())
	}
	if mc["sub"] != "7" || mc["role"] != "member" || mc["iss"] != "tasktracker" {
		t.Errorf("claims = %v", mc)
	}
	if exp, _ := mc["exp"].(float64); int64(exp) != t0.Add(time.Hour).Unix() {
		t.Errorf("exp = %v, want now+TTL", mc["exp"])
	}
	if iat, _ := mc["iat"].(float64); int64(iat) != t0.Unix() {
		t.Errorf("iat = %v, want now", mc["iat"])
	}
}

func TestShortSecretIsRefused(t *testing.T) {
	now := t0
	svc := newService(&now)
	svc.Secret = []byte("too short")
	if _, err := svc.Issue(Claims{UserID: 1}); err == nil {
		t.Fatal("Issue accepted a short secret")
	}
}

func TestExpiry(t *testing.T) {
	now := t0
	svc := newService(&now)
	token, err := svc.Issue(Claims{UserID: 7, Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	now = t0.Add(59 * time.Minute)
	if _, err := svc.Verify(token); err != nil {
		t.Fatalf("still valid, got %v", err)
	}
	now = t0.Add(61 * time.Minute)
	_, err = svc.Verify(token)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
	if errors.Is(err, ErrInvalid) {
		t.Fatal("an expired token must not also be reported as invalid")
	}
}

func TestRejectsUntrustworthyTokens(t *testing.T) {
	now := t0
	svc := newService(&now)
	good, err := svc.Issue(Claims{UserID: 7, Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(good, ".")

	noExp := validClaims()
	delete(noExp, "exp")
	wrongIssuer := validClaims()
	wrongIssuer["iss"] = "evil"
	badSubject := validClaims()
	badSubject["sub"] = "not-a-number"

	tests := map[string]string{
		"empty":             "",
		"garbage":           "abc.def.ghi",
		"tampered payload":  parts[0] + "." + parts[1][:len(parts[1])-2] + "AA." + parts[2],
		"wrong secret":      sign(t, jwt.SigningMethodHS256, []byte("another-secret-another-secret-123"), validClaims()),
		"alg none":          sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, validClaims()),
		"other HMAC alg":    sign(t, jwt.SigningMethodHS512, secret, validClaims()),
		"wrong issuer":      sign(t, jwt.SigningMethodHS256, secret, wrongIssuer),
		"no expiry":         sign(t, jwt.SigningMethodHS256, secret, noExp),
		"non-numeric user":  sign(t, jwt.SigningMethodHS256, secret, badSubject),
		"missing signature": parts[0] + "." + parts[1],
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Verify(token); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
	if _, err := svc.Verify(sign(t, jwt.SigningMethodHS256, secret, validClaims())); err != nil {
		t.Fatalf("control token should be valid: %v", err)
	}
}

func TestZeroNowUsesRealClock(t *testing.T) {
	svc := Service{Secret: secret, Issuer: "tasktracker", TTL: time.Hour}
	token, err := svc.Issue(Claims{UserID: 3, Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Verify(token); err != nil || got.UserID != 3 {
		t.Fatalf("Verify = %+v, %v", got, err)
	}
}
