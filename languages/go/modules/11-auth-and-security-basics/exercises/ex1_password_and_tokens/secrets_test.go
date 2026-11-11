package secrets

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

var testHasher = Hasher{Cost: bcrypt.MinCost}

func TestHashIsSaltedAndNotPlaintext(t *testing.T) {
	h1, err := testHasher.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := testHasher.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h1, "correct horse") || !strings.HasPrefix(h1, "$2") {
		t.Fatalf("hash %q does not look like bcrypt", h1)
	}
	if h1 == h2 {
		t.Fatal("two hashes of the same password are identical: no salt")
	}
}

func TestVerify(t *testing.T) {
	hash, err := testHasher.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if err := testHasher.Verify(hash, "correct horse"); err != nil {
		t.Fatalf("right password rejected: %v", err)
	}
	if err := testHasher.Verify(hash, "wrong horse"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("wrong password: err = %v, want ErrMismatch", err)
	}
	err = testHasher.Verify("not-a-hash", "whatever")
	if err == nil || errors.Is(err, ErrMismatch) {
		t.Fatalf("malformed hash: err = %v, want a non-mismatch error", err)
	}
}

func TestPasswordRules(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     error
	}{
		{"too short", "1234567", ErrWeakPassword},
		{"shortest allowed", "12345678", nil},
		{"longest allowed", strings.Repeat("x", 72), nil},
		{"too long", strings.Repeat("x", 73), ErrPasswordTooLong},
		{"multibyte counts bytes", strings.Repeat("é", 37), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testHasher.Hash(tt.password)
			if !errors.Is(err, tt.want) || (tt.want == nil && err != nil) {
				t.Fatalf("Hash err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	old, err := Hasher{Cost: bcrypt.MinCost}.Hash("password1")
	if err != nil {
		t.Fatal(err)
	}
	stronger := Hasher{Cost: bcrypt.MinCost + 1}
	if !stronger.NeedsRehash(old) {
		t.Error("a hash with a lower cost should need rehashing")
	}
	if testHasher.NeedsRehash(old) {
		t.Error("a hash with the configured cost should not need rehashing")
	}
	if !testHasher.NeedsRehash("garbage") {
		t.Error("an invalid hash should need rehashing")
	}
}

func TestNewToken(t *testing.T) {
	hex := regexp.MustCompile(`^[0-9a-f]+$`)
	seen := map[string]bool{}
	for range 50 {
		tok, err := NewToken(16)
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 || !hex.MatchString(tok) {
			t.Fatalf("token %q: want 32 hex characters", tok)
		}
		if seen[tok] {
			t.Fatalf("token %q repeated", tok)
		}
		seen[tok] = true
	}
	for _, n := range []int{0, -1} {
		if _, err := NewToken(n); err == nil {
			t.Errorf("NewToken(%d) should fail", n)
		}
	}
}

func TestHashToken(t *testing.T) {
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashToken("abc"); got != want {
		t.Fatalf("HashToken(abc) = %q", got)
	}
}

func TestTokenEqual(t *testing.T) {
	if !TokenEqual("abc123", "abc123") {
		t.Error("equal tokens reported different")
	}
	for _, other := range []string{"abc124", "abc12", "", "abc1234"} {
		if TokenEqual("abc123", other) {
			t.Errorf("TokenEqual(abc123, %q) = true", other)
		}
	}
}
