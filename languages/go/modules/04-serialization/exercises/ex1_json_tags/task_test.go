package jsontags

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

var created = time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

func TestEncodeGolden(t *testing.T) {
	due, owner := "2030-01-31", 7
	tests := []struct {
		name string
		in   Task
		want string
	}{
		{
			"nil tags become empty array, nil pointers become null",
			Task{ID: 1, Title: "Write docs", Status: "todo", CreatedAt: created, Internal: "secret"},
			`{"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}`,
		},
		{
			"all fields set",
			Task{ID: 2, Title: "Ship", Status: "done", DueDate: &due, Tags: []string{"docs", "p1"}, OwnerID: &owner, CreatedAt: created},
			`{"id":2,"title":"Ship","status":"done","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":7,"created_at":"2030-01-01T09:00:00Z"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestEncodeDoesNotMutateInput(t *testing.T) {
	in := Task{Title: "x"}
	if _, err := Encode(in); err != nil {
		t.Fatal(err)
	}
	if in.Tags != nil {
		t.Fatal("Encode modified the caller's Tags")
	}
}

func TestDecode(t *testing.T) {
	got, err := Decode([]byte(`{"id":3,"title":"T","status":"in_progress","due_date":"2030-02-01","tags":["a"],"owner_id":9,"created_at":"2030-01-01T09:00:00Z","internal":"nope","Internal":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 3 || got.Status != "in_progress" || got.DueDate == nil || *got.DueDate != "2030-02-01" ||
		got.OwnerID == nil || *got.OwnerID != 9 || !slices.Equal(got.Tags, []string{"a"}) || !got.CreatedAt.Equal(created) {
		t.Fatalf("unexpected decode result: %+v", got)
	}
	if got.Internal != "" {
		t.Fatalf("Internal must not be decoded, got %q", got.Internal)
	}
}

func TestRoundTrip(t *testing.T) {
	in := Task{ID: 5, Title: "Round", Status: "todo", Tags: []string{"x"}, CreatedAt: created}
	b, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]any
	if err := json.Unmarshal(b, &generic); err != nil {
		t.Fatal(err)
	}
	if _, ok := generic["Title"]; ok {
		t.Fatal("keys must be snake_case, found Title")
	}
	out, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != in.Title || out.ID != in.ID || !out.CreatedAt.Equal(in.CreatedAt) {
		t.Fatalf("round trip mismatch: %+v", out)
	}
}
