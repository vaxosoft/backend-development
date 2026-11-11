package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

// files lists every regular file below dir (slash-separated, relative), including hidden temp files.
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveStoresContentAndReportsSizeAndHash(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir, MaxBytes: 1 << 20}

	info, err := s.Save("notes/today.txt", strings.NewReader("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("hello world"))
	if info.Size != 11 || info.SHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("info = %+v", info)
	}
	if got := readFile(t, filepath.Join(dir, "notes", "today.txt")); got != "hello world" {
		t.Errorf("content = %q", got)
	}
	if got := files(t, dir); !slices.Equal(got, []string{"notes/today.txt"}) {
		t.Errorf("files = %v: no temporary files may remain", got)
	}

	f, err := s.Open("notes/today.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if b, _ := io.ReadAll(f); string(b) != "hello world" {
		t.Errorf("Open content = %q", b)
	}
}

func TestUnsafeNamesAreRejectedBeforeAnythingIsWritten(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "store")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: dir, MaxBytes: 100}

	for _, name := range []string{"", ".", "..", "../evil.txt", "../../etc/passwd", "a/../../evil.txt", "/etc/passwd", "/abs/path.txt"} {
		if _, err := s.Save(name, strings.NewReader("x")); !errors.Is(err, ErrUnsafeName) {
			t.Errorf("Save(%q) err = %v, want ErrUnsafeName", name, err)
		}
		if _, err := s.Open(name); !errors.Is(err, ErrUnsafeName) {
			t.Errorf("Open(%q) err = %v, want ErrUnsafeName", name, err)
		}
	}
	if got := files(t, parent); len(got) != 0 {
		t.Errorf("files created: %v", got)
	}
}

func TestTooLargeLeavesNothingBehindAndKeepsOldContent(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir, MaxBytes: 10}

	if _, err := s.Save("a.txt", strings.NewReader("0123456789")); err != nil {
		t.Fatalf("exactly MaxBytes must be accepted: %v", err)
	}
	if _, err := s.Save("a.txt", strings.NewReader("0123456789X")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if got := readFile(t, filepath.Join(dir, "a.txt")); got != "0123456789" {
		t.Errorf("existing file was clobbered: %q", got)
	}
	if _, err := s.Save("b.txt", strings.NewReader(strings.Repeat("z", 100))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if got := files(t, dir); !slices.Equal(got, []string{"a.txt"}) {
		t.Errorf("files = %v, want only a.txt", got)
	}
}

func TestOversizedUploadIsNotReadToTheEnd(t *testing.T) {
	s := &Store{Dir: t.TempDir(), MaxBytes: 1000}
	cr := &countingReader{r: strings.NewReader(strings.Repeat("a", 10<<20))}
	if _, err := s.Save("big.bin", cr); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
	if cr.n > 64<<10 {
		t.Errorf("read %d bytes from an upload limited to 1000: stop at MaxBytes+1", cr.n)
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

func TestFailedReadKeepsOldFileAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir, MaxBytes: 1 << 20}
	if _, err := s.Save("data.txt", strings.NewReader("old")); err != nil {
		t.Fatal(err)
	}

	boom := errors.New("connection dropped")
	_, err := s.Save("data.txt", io.MultiReader(strings.NewReader("partial new"), iotest.ErrReader(boom)))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if got := readFile(t, filepath.Join(dir, "data.txt")); got != "old" {
		t.Errorf("content = %q, want the old content", got)
	}
	if got := files(t, dir); !slices.Equal(got, []string{"data.txt"}) {
		t.Errorf("files = %v", got)
	}
}

func TestOpenMissingFile(t *testing.T) {
	s := &Store{Dir: t.TempDir(), MaxBytes: 10}
	_, err := s.Open("nope.txt")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
