package task

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateValidTaskReturnsTrueNil(t *testing.T) {
	err := Validate(Task{Title: "  ok  ", Status: "done", Tags: []string{"a"}})
	if err != nil {
		t.Fatalf("want a nil error interface, got %#v", err)
	}
	if err := Validate(Task{Title: "x"}); err != nil {
		t.Fatalf("empty status must be allowed, got %v", err)
	}
}

func TestValidateReportsEveryBrokenField(t *testing.T) {
	err := Validate(Task{Title: "   ", Status: "blocked", Tags: make([]string, 11)})

	var fe *FieldErrors
	if !errors.As(err, &fe) {
		t.Fatalf("want *FieldErrors, got %T", err)
	}
	want := map[string]string{
		"title":  "must be 1-200 characters",
		"status": "must be todo, in_progress or done",
		"tags":   "at most 10 tags",
	}
	if len(fe.Fields) != len(want) {
		t.Fatalf("fields = %v, want %v", fe.Fields, want)
	}
	for k, v := range want {
		if fe.Fields[k] != v {
			t.Errorf("field %q = %q, want %q", k, fe.Fields[k], v)
		}
	}
}

func TestValidateTitleBoundaries(t *testing.T) {
	tests := []struct {
		title   string
		invalid bool
	}{
		{"", true},
		{strings.Repeat("a", 200), false},
		{strings.Repeat("a", 201), true},
		{" " + strings.Repeat("a", 200) + " ", false}, // trimmed before measuring
	}
	for _, tt := range tests {
		err := Validate(Task{Title: tt.title})
		if (err != nil) != tt.invalid {
			t.Errorf("title len %d: err = %v, invalid = %v", len(tt.title), err, tt.invalid)
		}
	}
}

func TestFieldErrorsMessageIsSortedAndMatchesSentinel(t *testing.T) {
	err := error(&FieldErrors{Fields: map[string]string{"title": "bad", "status": "worse"}})
	if got, want := err.Error(), "validation failed: status: worse; title: bad"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, ErrValidation) {
		t.Error("errors.Is(err, ErrValidation) = false")
	}
	if !errors.Is(fmt.Errorf("create: %w", err), ErrValidation) {
		t.Error("the match must survive wrapping")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("must not match unrelated sentinels")
	}
}

func TestParseStatus(t *testing.T) {
	for _, ok := range []string{"todo", "in_progress", "done"} {
		if got, err := ParseStatus(ok); err != nil || got != ok {
			t.Errorf("ParseStatus(%q) = %q, %v", ok, got, err)
		}
	}
	_, err := ParseStatus("DONE")
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("want ErrInvalidStatus, got %v", err)
	}
	if !strings.Contains(err.Error(), `"DONE"`) {
		t.Errorf("message should quote the input: %v", err)
	}
}

func TestRepoGet(t *testing.T) {
	repo := NewRepo(Task{ID: 1, Title: "one"})
	got, err := repo.Get(1)
	if err != nil || got.Title != "one" {
		t.Fatalf("Get(1) = %+v, %v", got, err)
	}
	_, err = repo.Get(7)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "7") {
		t.Errorf("message should mention the id: %v", err)
	}
}
