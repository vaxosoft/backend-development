package datetype

import (
	"encoding/json"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) *Date {
	return &Date{time.Date(y, m, d, 0, 0, 0, 0, time.UTC)}
}

func TestMarshalTask(t *testing.T) {
	tests := []struct {
		name string
		in   Task
		want string
	}{
		{"with date", Task{Title: "a", Status: InProgress, DueDate: date(2030, 1, 31)}, `{"title":"a","status":"in_progress","due_date":"2030-01-31"}`},
		{"nil date is null", Task{Title: "b", Status: Done}, `{"title":"b","status":"done","due_date":null}`},
		{"zero value status", Task{Title: "c"}, `{"title":"c","status":"todo","due_date":null}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMarshalInvalidStatus(t *testing.T) {
	if _, err := json.Marshal(Task{Status: Status(42)}); err == nil {
		t.Fatal("expected an error for an out-of-range status")
	}
}

func TestUnmarshalTask(t *testing.T) {
	var got Task
	if err := json.Unmarshal([]byte(`{"title":"x","status":"done","due_date":"2030-02-28"}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != Done || got.DueDate == nil || !got.DueDate.Equal(date(2030, 2, 28).Time) {
		t.Fatalf("unexpected result %+v", got)
	}

	got = Task{DueDate: date(2000, 1, 1)}
	if err := json.Unmarshal([]byte(`{"title":"y","due_date":null}`), &got); err != nil {
		t.Fatal(err)
	}
	if got.DueDate != nil {
		t.Fatalf("null should reset the pointer, got %v", got.DueDate)
	}
}

func TestUnmarshalRejectsBadInput(t *testing.T) {
	bad := map[string]string{
		"impossible date": `{"due_date":"2030-02-30"}`,
		"wrong layout":    `{"due_date":"31/01/2030"}`,
		"time included":   `{"due_date":"2030-01-31T10:00:00Z"}`,
		"number":          `{"due_date":20300131}`,
		"unknown status":  `{"status":"later"}`,
		"status number":   `{"status":1}`,
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			var got Task
			if err := json.Unmarshal([]byte(body), &got); err == nil {
				t.Fatalf("expected an error, got %+v", got)
			}
		})
	}
}

func TestStatusAsMapKey(t *testing.T) {
	b, err := json.Marshal(map[Status]int{Todo: 2, Done: 1})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"done":1,"todo":2}` {
		t.Fatalf("got %s", b)
	}
	var back map[Status]int
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back[Todo] != 2 || back[Done] != 1 {
		t.Fatalf("got %v", back)
	}
}
