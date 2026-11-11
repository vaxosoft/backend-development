package starter

import "time"

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
	// TODO: implement the rules above.
	return nil
}
