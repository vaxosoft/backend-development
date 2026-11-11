package titles

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// assertEqual is a tiny generic helper: t.Helper makes failures point at the caller's line.
func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "Write docs", "write-docs"},
		{"punctuation collapses", "  Fix: the *bug*!  ", "fix-the-bug"},
		{"digits are kept", "Release v2.0", "release-v2-0"},
		{"unicode letters", "Café Müller", "café-müller"},
		{"only symbols", "!!!", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // safe: every case only reads its own tt (a fresh variable per iteration since Go 1.22)
			assertEqual(t, Slug(tt.in), tt.want)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"trims", "  Write docs  ", "Write docs", nil},
		{"empty", "", "", ErrEmpty},
		{"blank", " \t\n", "", ErrEmpty},
		{"200 runes ok", strings.Repeat("é", 200), strings.Repeat("é", 200), nil},
		{"201 runes too long", strings.Repeat("é", 201), "", ErrTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Validate(tt.in)
			// Compare errors with errors.Is, never by message text.
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			assertEqual(t, got, tt.want)
		})
	}
}

func TestTags(t *testing.T) {
	in := []string{"Docs", " p1 ", "docs", "", "  ", "P1", "bug"}
	want := []string{"docs", "p1", "bug"}
	if got := Tags(in); !slices.Equal(got, want) {
		t.Fatalf("Tags(%q) = %q, want %q", in, got, want)
	}
	if got := Tags(nil); got == nil || len(got) != 0 {
		t.Fatalf("Tags(nil) = %#v, want an empty non-nil slice (it marshals to [] not null)", got)
	}
}
