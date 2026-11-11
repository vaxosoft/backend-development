package patch

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
)

func decode(t *testing.T, body string) Patch {
	t.Helper()
	p, err := Decode(strings.NewReader(body))
	if err != nil {
		t.Fatalf("Decode(%q): %v", body, err)
	}
	return p
}

func TestDecodeTriState(t *testing.T) {
	p := decode(t, `{"due_date":null,"title":"New"}`)
	if !p.DueDate.Set || !p.DueDate.Null {
		t.Fatalf("due_date: want set+null, got %+v", p.DueDate)
	}
	if !p.Title.Set || p.Title.Null || p.Title.Value != "New" {
		t.Fatalf("title: got %+v", p.Title)
	}
	if p.Status.Set || p.Tags.Set {
		t.Fatalf("absent keys must not be Set: %+v %+v", p.Status, p.Tags)
	}
	if p := decode(t, `{"tags":["a","b"]}`); !slices.Equal(p.Tags.Value, []string{"a", "b"}) {
		t.Fatalf("tags: %+v", p.Tags)
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := map[string]string{
		"empty":         ``,
		"malformed":     `{"title":`,
		"unknown field": `{"colour":"red"}`,
		"read-only id":  `{"id":5}`,
		"created_at":    `{"created_at":"2030-01-01T00:00:00Z"}`,
		"wrong type":    `{"title":5}`,
		"not an object": `[1,2]`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(strings.NewReader(body)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
	_, err := Decode(strings.NewReader(`{"title":"a"} {"title":"b"}`))
	if !errors.Is(err, ErrTrailingData) {
		t.Fatalf("want ErrTrailingData, got %v", err)
	}
}

func base() Task {
	due := "2030-01-31"
	return Task{ID: 1, Title: "Old", Status: "todo", DueDate: &due, Tags: []string{"x"}}
}

func TestApplyChangesOnlyPresentFields(t *testing.T) {
	got, fields := decode(t, `{"status":"done"}`).Apply(base())
	if len(fields) != 0 {
		t.Fatal(fields)
	}
	want := base()
	want.Status = "done"
	if got.Title != want.Title || got.Status != "done" || *got.DueDate != *want.DueDate || !slices.Equal(got.Tags, want.Tags) || got.ID != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyNullClears(t *testing.T) {
	got, fields := decode(t, `{"due_date":null,"tags":null}`).Apply(base())
	if len(fields) != 0 {
		t.Fatal(fields)
	}
	if got.DueDate != nil {
		t.Fatalf("due_date should be cleared, got %v", *got.DueDate)
	}
	if got.Tags == nil || len(got.Tags) != 0 {
		t.Fatalf("tags should be an empty non-nil slice, got %#v", got.Tags)
	}
}

func TestApplyTrimsTitleAndSetsDate(t *testing.T) {
	got, fields := decode(t, `{"title":"  Better  ","due_date":"2031-12-01"}`).Apply(base())
	if len(fields) != 0 {
		t.Fatal(fields)
	}
	if got.Title != "Better" || *got.DueDate != "2031-12-01" {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyValidation(t *testing.T) {
	tests := []struct {
		name, body, field string
	}{
		{"null title", `{"title":null}`, "title"},
		{"blank title", `{"title":"   "}`, "title"},
		{"long title", `{"title":"` + strings.Repeat("a", 201) + `"}`, "title"},
		{"null status", `{"status":null}`, "status"},
		{"bad status", `{"status":"later"}`, "status"},
		{"bad date", `{"due_date":"2030-02-30"}`, "due_date"},
		{"uppercase tag", `{"tags":["Docs"]}`, "tags"},
		{"duplicate tag", `{"tags":["a","a"]}`, "tags"},
		{"empty tag", `{"tags":[""]}`, "tags"},
		{"long tag", `{"tags":["` + strings.Repeat("a", 31) + `"]}`, "tags"},
		{"too many tags", `{"tags":["a","b","c","d","e","f","g","h","i","j","k"]}`, "tags"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := base()
			got, fields := decode(t, tt.body).Apply(orig)
			if fields[tt.field] == "" {
				t.Fatalf("want error for %q, got %v", tt.field, fields)
			}
			if got.Title != orig.Title || got.Status != orig.Status {
				t.Fatalf("task must be returned unchanged on error, got %+v", got)
			}
		})
	}
}

func TestApplyReportsAllFields(t *testing.T) {
	_, fields := decode(t, `{"title":"","status":"x","due_date":"nope"}`).Apply(base())
	got := slices.Sorted(maps.Keys(fields))
	if !slices.Equal(got, []string{"due_date", "status", "title"}) {
		t.Fatalf("got %v", got)
	}
}

func TestApplyDoesNotAliasPatchTags(t *testing.T) {
	p := decode(t, `{"tags":["a","b"]}`)
	got, _ := p.Apply(base())
	got.Tags[0] = "changed"
	if p.Tags.Value[0] != "a" {
		t.Fatal("Apply must copy the tags slice")
	}
}
