// Package ndjson is exercise 3: gzip-compressed newline-delimited JSON, written and read as streams.
package ndjson

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
)

// Task is one record.
type Task struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// LineError reports a record that is not valid JSON; Line is 1-based and counts blank lines too.
type LineError struct {
	Line int
	Err  error
}

func (e *LineError) Error() string { return fmt.Sprintf("line %d: %v", e.Line, e.Err) }
func (e *LineError) Unwrap() error { return e.Err }

// Write compresses the tasks to w as gzip(NDJSON): one JSON object per line, each ending in "\n".
//
// Tasks are consumed one at a time (no slice of all lines). Write does not close w, but it must
// finish the gzip stream, and every error from w, including the one reported when the gzip stream
// is finished, must be returned.
func Write(w io.Writer, tasks iter.Seq[Task]) error {
	zw := gzip.NewWriter(w)
	enc := json.NewEncoder(zw)
	for t := range tasks {
		if err := enc.Encode(t); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}

// Read decompresses r and yields the tasks one by one.
//
//   - Blank (whitespace-only) lines are skipped; the last line may lack a trailing "\n".
//   - Lines may be arbitrarily long (no 64 KiB token limit).
//   - An invalid JSON line yields (Task{}, *LineError) and ends the iteration.
//   - A corrupt or truncated gzip stream yields an error that wraps the cause (for a truncated
//     stream io.ErrUnexpectedEOF, for a bad header gzip.ErrHeader) and ends the iteration. A partial
//     last line from a truncated stream must not be decoded as if it were complete.
//   - When the consumer stops early, stop reading from r.
func Read(r io.Reader) iter.Seq2[Task, error] {
	return func(yield func(Task, error) bool) {
		zr, err := gzip.NewReader(r)
		if err != nil {
			yield(Task{}, fmt.Errorf("open gzip: %w", err))
			return
		}
		defer zr.Close()

		br := bufio.NewReader(zr)
		for line := 1; ; line++ {
			raw, err := br.ReadBytes('\n')
			atEOF := errors.Is(err, io.EOF)
			if err != nil && !atEOF {
				yield(Task{}, fmt.Errorf("read gzip: %w", err))
				return
			}
			if raw = bytes.TrimSpace(raw); len(raw) > 0 {
				var t Task
				if err := json.Unmarshal(raw, &t); err != nil {
					yield(Task{}, &LineError{Line: line, Err: err})
					return
				}
				if !yield(t, nil) {
					return
				}
			}
			if atEOF {
				return
			}
		}
	}
}
