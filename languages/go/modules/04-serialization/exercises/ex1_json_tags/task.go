// Package jsontags is exercise 1: make Task match the wire format of the Task Tracker API.
package jsontags

import (
	"encoding/json"
	"time"
)

// Task must serialize as:
//
//	{"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
//
// Internal must never appear in JSON.
type Task struct {
	ID        int
	Title     string
	Status    string
	DueDate   *string
	Tags      []string
	OwnerID   *int
	CreatedAt time.Time
	Internal  string
}

// Encode returns the JSON for t. A nil Tags slice must be sent as [] and t must not be modified.
func Encode(t Task) ([]byte, error) {
	return json.Marshal(t)
}

// Decode parses the JSON produced by Encode.
func Decode(data []byte) (Task, error) {
	var t Task
	err := json.Unmarshal(data, &t)
	return t, err
}
