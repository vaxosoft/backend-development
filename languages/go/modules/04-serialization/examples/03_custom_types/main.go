// Custom (un)marshalling: a Date type, an enum via TextMarshaler, RawMessage and time.Time.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Date is a calendar date serialized as "YYYY-MM-DD".
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(dateLayout))
}

func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("due_date must be a string: %w", err)
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return fmt.Errorf("due_date must be YYYY-MM-DD: %w", err)
	}
	d.Time = t
	return nil
}

type Status int

const (
	Todo Status = iota
	InProgress
	Done
)

var statusNames = [...]string{"todo", "in_progress", "done"}

func (s Status) MarshalText() ([]byte, error) {
	if int(s) < 0 || int(s) >= len(statusNames) {
		return nil, fmt.Errorf("invalid status %d", int(s))
	}
	return []byte(statusNames[s]), nil
}

func (s *Status) UnmarshalText(b []byte) error {
	for i, n := range statusNames {
		if n == string(b) {
			*s = Status(i)
			return nil
		}
	}
	return fmt.Errorf("unknown status %q", b)
}

type Task struct {
	Title     string    `json:"title"`
	Status    Status    `json:"status"`
	DueDate   *Date     `json:"due_date"`
	CreatedAt time.Time `json:"created_at"`
}

// Event keeps its payload undecoded until the type is known.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	due := Date{time.Date(2030, 1, 31, 0, 0, 0, 0, time.UTC)}
	t := Task{
		Title:     "Write docs",
		Status:    InProgress,
		DueDate:   &due,
		CreatedAt: time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC),
	}
	b, err := json.Marshal(t)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	var back Task
	if err := json.Unmarshal(b, &back); err != nil {
		log.Fatal(err)
	}
	fmt.Println(back.Status == InProgress, back.DueDate.Format(dateLayout), back.CreatedAt.Equal(t.CreatedAt))

	for _, bad := range []string{
		`{"title":"x","status":"later"}`,
		`{"title":"x","due_date":"31/01/2030"}`,
		`{"title":"x","due_date":20300131}`,
	} {
		var t Task
		fmt.Println(json.Unmarshal([]byte(bad), &t))
	}

	// Enums as map keys work too, because TextMarshaler is used for keys.
	counts, _ := json.Marshal(map[Status]int{Todo: 3, Done: 1})
	fmt.Println(string(counts))

	var ev Event
	if err := json.Unmarshal([]byte(`{"type":"task.done","payload":{"id":4}}`), &ev); err != nil {
		log.Fatal(err)
	}
	if ev.Type == "task.done" {
		var p struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			log.Fatal(err)
		}
		fmt.Println(ev.Type, p.ID, string(ev.Payload))
	}
}
