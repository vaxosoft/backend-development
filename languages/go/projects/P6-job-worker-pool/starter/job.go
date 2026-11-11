package starter

import (
	"context"
	"encoding/json"
	"errors"
)

// Job types accepted by POST /jobs.
const (
	TypeReverse = "reverse"
	TypeSleep   = "sleep"
	TypeFail    = "fail"
)

// MaxSleepMS is the largest allowed "sleep" payload.
const MaxSleepMS = 5000

// Status is the lifecycle state of a job.
type Status string

// Job statuses.
const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Finished reports whether the job can no longer change state.
func (s Status) Finished() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCancelled
}

// JobView is the JSON representation of a job.
type JobView struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	Status   Status  `json:"status"`
	Attempts int     `json:"attempts"`
	Result   *string `json:"result"`
	Error    *string `json:"error"`
}

var errDeliberate = errors.New("job failed on purpose")

type reversePayload struct {
	Text *string `json:"text"`
}

type sleepPayload struct {
	MS *int `json:"ms"`
}

// validate returns a field -> message map; it is empty when the job is valid.
// Field names look like "type", "payload.text" or "payload.ms".
func validate(_ string, _ json.RawMessage) map[string]string {
	// TODO: reject unknown types, a reverse payload without a string "text"
	// and a sleep payload whose "ms" is missing or outside 0..MaxSleepMS.
	return map[string]string{}
}

// execute runs one attempt of a job. It must return promptly when ctx ends.
func execute(_ context.Context, _ string, _ json.RawMessage) (string, error) {
	// TODO: reverse -> reversed text (by rune); sleep -> wait "ms" milliseconds
	// or until ctx ends; fail -> errDeliberate.
	return "", errors.New("TODO: execute is not implemented")
}
