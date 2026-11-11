package starter

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

var (
	// ErrNotFound means no file has this id (or the id is malformed).
	ErrNotFound = errors.New("file not found")
	// ErrTooLarge means the upload exceeded the size limit.
	ErrTooLarge = errors.New("file too large")
)

var errTodo = errors.New("TODO: not implemented")

// Meta describes a stored file and is the JSON returned by POST /files.
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"content_type"`
}

// Storage keeps files in one directory under generated ids. Suggested layout:
// <id>.blob holds the bytes, <id>.json holds the Meta. Uploads are written to a
// temporary file in the same directory and renamed into place, so a reader
// never sees a half-written file.
type Storage struct {
	dir string
}

// NewStorage creates dir if needed and returns a Storage on it.
func NewStorage(dir string) (*Storage, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return &Storage{dir: dir}, nil
}

// NewID returns a random 32-character lowercase hex id.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Save streams r to disk while hashing it, never holding the whole file in
// memory. It reads at most limit bytes: if r has more, it stores nothing and
// returns ErrTooLarge. No file (temporary or final) may remain after an error.
// name must already be sanitised.
func (s *Storage) Save(r io.Reader, name, contentType string, limit int64) (Meta, error) {
	// TODO: os.CreateTemp, io.Copy(io.MultiWriter(file, hasher), io.LimitReader(r, limit+1)),
	// check the byte count, check Close's error, rename, write the metadata.
	return Meta{}, errTodo
}

// Open returns the data file and its metadata. A malformed id (anything but
// 32 lowercase hex characters) is ErrNotFound: it must never reach the
// filesystem. The caller closes the file.
func (s *Storage) Open(id string) (*os.File, Meta, error) {
	return nil, Meta{}, errTodo
}

// Delete removes the file or returns ErrNotFound.
func (s *Storage) Delete(id string) error {
	return errTodo
}
