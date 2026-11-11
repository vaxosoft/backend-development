package envparse

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"
)

func env(kv map[string]string) Lookup {
	return func(k string) (string, bool) { v, ok := kv[k]; return v, ok }
}

func TestStringAndDefaults(t *testing.T) {
	l := env(map[string]string{"SET": "  value  ", "EMPTY": "", "BLANK": "   "})
	for key, want := range map[string]string{"SET": "value", "EMPTY": "fallback", "BLANK": "fallback", "MISSING": "fallback"} {
		if got := String(l, key, "fallback"); got != want {
			t.Errorf("String(%s) = %q, want %q", key, got, want)
		}
	}
}

func TestInt(t *testing.T) {
	l := env(map[string]string{"A": "42", "B": " -7 ", "EMPTY": "", "BAD": "eighty", "BIG": "99999999999999999999"})
	if n, err := Int(l, "A", 1); n != 42 || err != nil {
		t.Errorf("A = %d, %v", n, err)
	}
	if n, err := Int(l, "B", 1); n != -7 || err != nil {
		t.Errorf("B = %d, %v", n, err)
	}
	if n, err := Int(l, "EMPTY", 8080); n != 8080 || err != nil {
		t.Errorf("EMPTY = %d, %v", n, err)
	}
	if n, err := Int(l, "MISSING", 8080); n != 8080 || err != nil {
		t.Errorf("MISSING = %d, %v", n, err)
	}

	n, err := Int(l, "BAD", 8080)
	var pe *ParseError
	if !errors.As(err, &pe) || pe.Key != "BAD" || pe.Value != "eighty" {
		t.Fatalf("err = %v, want *ParseError for BAD", err)
	}
	if n != 0 {
		t.Errorf("n = %d, want zero value on error", n)
	}
	if !errors.Is(err, strconv.ErrSyntax) {
		t.Error("the strconv cause must stay reachable with errors.Is")
	}
	if _, err := Int(l, "BIG", 1); !errors.Is(err, strconv.ErrRange) {
		t.Errorf("BIG: err = %v, want ErrRange", err)
	}
}

func TestBool(t *testing.T) {
	l := env(map[string]string{"T1": "true", "T2": "1", "F1": "FALSE", "F2": " f ", "BAD": "yes"})
	for key, want := range map[string]bool{"T1": true, "T2": true, "F1": false, "F2": false} {
		if got, err := Bool(l, key, !want); got != want || err != nil {
			t.Errorf("Bool(%s) = %v, %v", key, got, err)
		}
	}
	if got, err := Bool(l, "MISSING", true); !got || err != nil {
		t.Errorf("default = %v, %v", got, err)
	}
	var pe *ParseError
	if _, err := Bool(l, "BAD", true); !errors.As(err, &pe) || pe.Key != "BAD" {
		t.Errorf("err = %v", err)
	}
}

func TestDuration(t *testing.T) {
	l := env(map[string]string{"A": "1500ms", "B": " 2m ", "NEG": "-1s", "BAD": "soon", "NOUNIT": "30"})
	if d, err := Duration(l, "A", 0); d != 1500*time.Millisecond || err != nil {
		t.Errorf("A = %v, %v", d, err)
	}
	if d, err := Duration(l, "B", 0); d != 2*time.Minute || err != nil {
		t.Errorf("B = %v, %v", d, err)
	}
	if d, err := Duration(l, "MISSING", 5*time.Second); d != 5*time.Second || err != nil {
		t.Errorf("default = %v, %v", d, err)
	}
	for _, key := range []string{"NEG", "BAD", "NOUNIT"} {
		d, err := Duration(l, key, time.Second)
		var pe *ParseError
		if !errors.As(err, &pe) || pe.Key != key || d != 0 {
			t.Errorf("%s: d=%v err=%v, want zero value and *ParseError", key, d, err)
		}
	}
}

func TestList(t *testing.T) {
	l := env(map[string]string{"A": "a, b ,,c,", "ONE": "solo", "BLANK": " ", "COMMAS": " , ,"})
	def := []string{"x"}
	cases := map[string][]string{"A": {"a", "b", "c"}, "ONE": {"solo"}, "BLANK": def, "MISSING": def, "COMMAS": nil}
	for key, want := range cases {
		if got := List(l, key, def); !slices.Equal(got, want) {
			t.Errorf("List(%s) = %q, want %q", key, got, want)
		}
	}
}
