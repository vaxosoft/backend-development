package ndjson

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"
	"testing"
)

func seq(tasks ...Task) iter.Seq[Task] { return slices.Values(tasks) }

func collect(r io.Reader) ([]Task, error) {
	var out []Task
	for t, err := range Read(r) {
		if err != nil {
			return out, err
		}
		out = append(out, t)
	}
	return out, nil
}

func gz(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := io.WriteString(zw, text); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRoundTrip(t *testing.T) {
	in := []Task{{1, "Write docs", "todo"}, {2, "Fix bug", "done"}, {3, "Line\nbreak \"quoted\"", "in_progress"}}
	var buf bytes.Buffer
	if err := Write(&buf, seq(in...)); err != nil {
		t.Fatal(err)
	}
	out, err := collect(&buf)
	if err != nil || !slices.Equal(out, in) {
		t.Fatalf("out = %v, err = %v", out, err)
	}
}

func TestWriteProducesOneJSONObjectPerLine(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, seq(Task{1, "a", "todo"}, Task{2, "b", "done"})); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("stream is not a complete gzip stream: %v", err)
	}
	want := `{"id":1,"title":"a","status":"todo"}` + "\n" + `{"id":2,"title":"b","status":"done"}` + "\n"
	if string(raw) != want {
		t.Errorf("payload = %q", raw)
	}
}

type failingWriter struct{ err error }

func (f failingWriter) Write([]byte) (int, error) { return 0, f.err }

func TestWriteReportsWriterErrors(t *testing.T) {
	boom := errors.New("disk full")
	// So little data that gzip only touches the writer when the stream is finished.
	if err := Write(failingWriter{boom}, seq(Task{1, "a", "todo"})); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestReadSkipsBlankLinesAndAcceptsMissingFinalNewline(t *testing.T) {
	text := `{"id":1,"title":"a","status":"todo"}` + "\n\n   \n" + `{"id":2,"title":"b","status":"done"}`
	out, err := collect(bytes.NewReader(gz(t, text)))
	if err != nil || len(out) != 2 || out[1].ID != 2 {
		t.Fatalf("out = %v, err = %v", out, err)
	}
}

func TestReadHandlesVeryLongLines(t *testing.T) {
	title := strings.Repeat("x", 1<<20)
	var buf bytes.Buffer
	if err := Write(&buf, seq(Task{1, title, "todo"})); err != nil {
		t.Fatal(err)
	}
	out, err := collect(&buf)
	if err != nil || len(out) != 1 || out[0].Title != title {
		t.Fatalf("long line failed: n=%d err=%v", len(out), err)
	}
}

func TestInvalidLineReportsLineNumberAndStops(t *testing.T) {
	text := `{"id":1,"title":"a","status":"todo"}` + "\n\n" + `{broken` + "\n" + `{"id":9,"title":"never","status":"todo"}` + "\n"
	out, err := collect(bytes.NewReader(gz(t, text)))
	var le *LineError
	if !errors.As(err, &le) || le.Line != 3 {
		t.Fatalf("err = %v, want *LineError at line 3", err)
	}
	if len(out) != 1 {
		t.Errorf("out = %v, want only the task before the bad line", out)
	}
}

func TestTruncatedStreamIsAnError(t *testing.T) {
	var buf bytes.Buffer
	var tasks []Task
	for i := range 200 {
		tasks = append(tasks, Task{i, fmt.Sprintf("task %d", i), "todo"})
	}
	if err := Write(&buf, seq(tasks...)); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	out, err := collect(bytes.NewReader(data[:len(data)-12]))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v, want io.ErrUnexpectedEOF", err)
	}
	if len(out) >= len(tasks) {
		t.Errorf("decoded %d tasks from a truncated stream", len(out))
	}
}

func TestNotGzipIsAnError(t *testing.T) {
	_, err := collect(strings.NewReader("plain text, not gzip"))
	if !errors.Is(err, gzip.ErrHeader) {
		t.Fatalf("err = %v, want gzip.ErrHeader", err)
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func TestReadStopsWhenConsumerBreaks(t *testing.T) {
	var buf bytes.Buffer
	all := func(yield func(Task) bool) {
		x := uint32(1)
		for i := range 20000 {
			x = x*1664525 + 1013904223 // pseudo-random title: compresses poorly, so the file is large
			if !yield(Task{i, fmt.Sprintf("%08x%08x", x, x*2654435761), "todo"}) {
				return
			}
		}
	}
	if err := Write(&buf, all); err != nil {
		t.Fatal(err)
	}
	total := buf.Len()

	cr := &countingReader{r: &buf}
	for _, err := range Read(cr) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if cr.n > total/4 {
		t.Errorf("read %d of %d compressed bytes after the consumer stopped after one task", cr.n, total)
	}
}
