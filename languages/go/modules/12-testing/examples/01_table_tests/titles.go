// Package titles is example 01: small pure functions are the easiest thing to test.
//
//	go test -v ./modules/12-testing/examples/01_table_tests/
package titles

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrEmpty   = errors.New("title is empty")
	ErrTooLong = errors.New("title is longer than 200 characters")
)

// Validate trims title and checks the Task Tracker rule: 1-200 characters.
func Validate(title string) (string, error) {
	title = strings.TrimSpace(title)
	switch n := utf8.RuneCountInString(title); {
	case n == 0:
		return "", ErrEmpty
	case n > 200:
		return "", ErrTooLong
	}
	return title, nil
}

// Slug turns a title into a lowercase, dash-separated URL fragment.
func Slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		} else {
			dash = true
		}
	}
	return b.String()
}

// Tags lowercases, trims and de-duplicates tags, keeping first-seen order and dropping empty ones.
func Tags(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
