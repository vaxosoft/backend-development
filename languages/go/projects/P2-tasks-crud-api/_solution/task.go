package starter

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Task statuses.
const (
	StatusTodo       = "todo"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
)

// Task is a stored task and its JSON wire format.
type Task struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	DueDate   *string   `json:"due_date"` // YYYY-MM-DD, null when unset
	Tags      []string  `json:"tags"`     // never null
	OwnerID   *int64    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TaskInput is the client-writable part of a Task (POST and PUT bodies).
type TaskInput struct {
	Title   string   `json:"title"`
	Status  string   `json:"status"`
	DueDate *string  `json:"due_date"`
	Tags    []string `json:"tags"`
	OwnerID *int64   `json:"owner_id"`
}

// Validate normalises in (trims the title, defaults the status to "todo",
// turns nil tags into an empty slice) and returns a message per invalid field
// keyed by its JSON name. An empty map means the input is valid.
//
// Rules: title 1-200 characters after trimming; status one of the constants;
// due_date a real calendar date in YYYY-MM-DD; at most 10 tags, each 1-30
// characters, lowercase, no duplicates.
func (in *TaskInput) Validate() map[string]string {
	problems := map[string]string{}

	in.Title = strings.TrimSpace(in.Title)
	if n := utf8.RuneCountInString(in.Title); n < 1 || n > 200 {
		problems["title"] = "must be 1-200 characters"
	}

	if in.Status == "" {
		in.Status = StatusTodo
	}
	switch in.Status {
	case StatusTodo, StatusInProgress, StatusDone:
	default:
		problems["status"] = "must be one of todo, in_progress, done"
	}

	if in.DueDate != nil {
		if _, err := time.Parse(time.DateOnly, *in.DueDate); err != nil {
			problems["due_date"] = "must be a date in YYYY-MM-DD format"
		}
	}

	if in.Tags == nil {
		in.Tags = []string{}
	}
	if msg := checkTags(in.Tags); msg != "" {
		problems["tags"] = msg
	}

	return problems
}

func checkTags(tags []string) string {
	if len(tags) > 10 {
		return "at most 10 tags"
	}
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if n := utf8.RuneCountInString(tag); n < 1 || n > 30 {
			return fmt.Sprintf("tag %q must be 1-30 characters", tag)
		}
		if tag != strings.ToLower(tag) {
			return fmt.Sprintf("tag %q must be lowercase", tag)
		}
		if seen[tag] {
			return fmt.Sprintf("tag %q is duplicated", tag)
		}
		seen[tag] = true
	}
	return ""
}
