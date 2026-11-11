package starter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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
func validate(jobType string, payload json.RawMessage) map[string]string {
	fields := map[string]string{}
	switch jobType {
	case TypeReverse:
		var p reversePayload
		if err := json.Unmarshal(payload, &p); err != nil || p.Text == nil {
			fields["payload.text"] = "is required and must be a string"
		}
	case TypeSleep:
		var p sleepPayload
		if err := json.Unmarshal(payload, &p); err != nil || p.MS == nil {
			fields["payload.ms"] = "is required and must be an integer"
		} else if *p.MS < 0 || *p.MS > MaxSleepMS {
			fields["payload.ms"] = fmt.Sprintf("must be between 0 and %d", MaxSleepMS)
		}
	case TypeFail:
	default:
		fields["type"] = "must be one of " + strings.Join([]string{TypeReverse, TypeSleep, TypeFail}, ", ")
	}
	return fields
}

// execute runs one attempt of a job. It must return promptly when ctx ends.
func execute(ctx context.Context, jobType string, payload json.RawMessage) (string, error) {
	switch jobType {
	case TypeReverse:
		var p reversePayload
		if err := json.Unmarshal(payload, &p); err != nil || p.Text == nil {
			return "", errors.New("invalid reverse payload")
		}
		r := []rune(*p.Text)
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		return string(r), nil
	case TypeSleep:
		var p sleepPayload
		if err := json.Unmarshal(payload, &p); err != nil || p.MS == nil {
			return "", errors.New("invalid sleep payload")
		}
		d := time.Duration(*p.MS) * time.Millisecond
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-timer.C:
			return fmt.Sprintf("slept %s", d), nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	case TypeFail:
		return "", errDeliberate
	}
	return "", fmt.Errorf("unknown job type %q", jobType)
}
