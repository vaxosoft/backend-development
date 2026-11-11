package wordcount

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
)

var cases = []struct {
	name string
	in   string
	want Stats
}{
	{"empty", "", Stats{}},
	{"one word no newline", "hello", Stats{Lines: 1, Words: 1, Bytes: 5}},
	{"trailing newline", "hello\n", Stats{Lines: 1, Words: 1, Bytes: 6}},
	{"several lines", "one two\nthree\n\nfour", Stats{Lines: 4, Words: 4, Bytes: 19}},
	{"only whitespace", "  \n\t \n", Stats{Lines: 2, Words: 0, Bytes: 6}},
	{"mixed separators", "a\tb  c\r\nd\ve\ff", Stats{Lines: 2, Words: 6, Bytes: 13}},
	{"utf-8 counts bytes", "héllo wörld\n", Stats{Lines: 1, Words: 2, Bytes: 14}},
}

func TestCount(t *testing.T) {
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Count(strings.NewReader(tc.in))
			if err != nil || got != tc.want {
				t.Fatalf("Count(%q) = %+v, %v; want %+v", tc.in, got, err, tc.want)
			}
		})
	}
}

func TestCountIsIndependentOfReadChunking(t *testing.T) {
	wrappers := map[string]func(io.Reader) io.Reader{
		"one byte at a time": iotest.OneByteReader,
		"data with EOF":      iotest.DataErrReader,
		"half reads":         iotest.HalfReader,
	}
	for wname, wrap := range wrappers {
		for _, tc := range cases {
			got, err := Count(wrap(strings.NewReader(tc.in)))
			if err != nil || got != tc.want {
				t.Errorf("%s: Count(%q) = %+v, %v; want %+v", wname, tc.in, got, err, tc.want)
			}
		}
	}
}

func TestCountReturnsReadErrors(t *testing.T) {
	boom := errors.New("disk on fire")
	r := io.MultiReader(strings.NewReader("abc def"), iotest.ErrReader(boom))
	if _, err := Count(r); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

// patternReader produces total bytes by repeating pattern, without ever storing them.
type patternReader struct {
	pattern string
	left    int
	off     int
}

func (p *patternReader) Read(b []byte) (int, error) {
	if p.left == 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(b) && p.left > 0 {
		b[n] = p.pattern[p.off]
		p.off = (p.off + 1) % len(p.pattern)
		n++
		p.left--
	}
	return n, nil
}

func TestCountHugeSingleLineInConstantMemory(t *testing.T) {
	const size = 9 * 5_000_000 // one 45 MB line: bufio.Scanner would fail, io.ReadAll would balloon
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	got, err := Count(&patternReader{pattern: "ab cd ef ", left: size})
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)

	if got.Bytes != size || got.Lines != 1 || got.Words != size/3 {
		t.Errorf("got %+v", got)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 8<<20 {
		t.Errorf("allocated %d MiB while counting: stream the input instead of buffering it", alloc>>20)
	}
}
