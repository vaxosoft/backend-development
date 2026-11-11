package starter

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	// ErrNotFound means no file has this id (or the id is malformed).
	ErrNotFound = errors.New("file not found")
	// ErrTooLarge means the upload exceeded the size limit.
	ErrTooLarge = errors.New("file too large")
)

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

func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (s *Storage) blobPath(id string) string { return filepath.Join(s.dir, id+".blob") }
func (s *Storage) metaPath(id string) string { return filepath.Join(s.dir, id+".json") }

// Save streams r to disk while hashing it, never holding the whole file in
// memory. It reads at most limit bytes: if r has more, it stores nothing and
// returns ErrTooLarge. No file (temporary or final) may remain after an error.
// name must already be sanitised.
func (s *Storage) Save(r io.Reader, name, contentType string, limit int64) (meta Meta, err error) {
	tmp, err := os.CreateTemp(s.dir, ".upload-*")
	if err != nil {
		return Meta{}, fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	hasher := sha256.New()
	// One byte more than the limit tells "exactly at the limit" from "over it".
	n, err := io.Copy(io.MultiWriter(tmp, hasher), io.LimitReader(r, limit+1))
	if err != nil {
		return Meta{}, fmt.Errorf("write upload: %w", err)
	}
	if n > limit {
		return Meta{}, ErrTooLarge
	}
	// Close is where a failed write on some filesystems is reported.
	if err = tmp.Close(); err != nil {
		return Meta{}, fmt.Errorf("close upload: %w", err)
	}

	meta = Meta{ID: NewID(), Name: name, Size: n, SHA256: hex.EncodeToString(hasher.Sum(nil)), ContentType: contentType}
	if err = os.Rename(tmp.Name(), s.blobPath(meta.ID)); err != nil {
		return Meta{}, fmt.Errorf("publish upload: %w", err)
	}
	raw, err := json.Marshal(meta)
	if err == nil {
		err = os.WriteFile(s.metaPath(meta.ID), raw, 0o600)
	}
	if err != nil {
		_ = os.Remove(s.blobPath(meta.ID))
		_ = os.Remove(s.metaPath(meta.ID))
		return Meta{}, fmt.Errorf("write metadata: %w", err)
	}
	return meta, nil
}

// Open returns the data file and its metadata. A malformed id (anything but
// 32 lowercase hex characters) is ErrNotFound: it must never reach the
// filesystem. The caller closes the file.
func (s *Storage) Open(id string) (*os.File, Meta, error) {
	if !validID(id) {
		return nil, Meta{}, ErrNotFound
	}
	raw, err := os.ReadFile(s.metaPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Meta{}, ErrNotFound
	}
	if err != nil {
		return nil, Meta{}, err
	}
	var meta Meta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, Meta{}, fmt.Errorf("decode metadata: %w", err)
	}
	f, err := os.Open(s.blobPath(id))
	if err != nil {
		return nil, Meta{}, err
	}
	return f, meta, nil
}

// Delete removes the file or returns ErrNotFound.
func (s *Storage) Delete(id string) error {
	if !validID(id) {
		return ErrNotFound
	}
	// The metadata file is what makes a file visible, so it goes first.
	if err := os.Remove(s.metaPath(id)); errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if err := os.Remove(s.blobPath(id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
