// Package store is exercise 2: a small upload store that streams, hashes, limits and never escapes its directory.
package store

import (
	"errors"
	"io"
	"os"
)

var (
	// ErrUnsafeName is returned for names that are not plain relative paths below the store directory.
	ErrUnsafeName = errors.New("unsafe name")
	// ErrTooLarge is returned when the content exceeds MaxBytes.
	ErrTooLarge = errors.New("content too large")
)

// Info describes a stored file.
type Info struct {
	Size   int64
	SHA256 string // lower-case hex
}

// Store keeps files below Dir.
type Store struct {
	Dir      string
	MaxBytes int64
}

// Save streams r into the file called name (a relative path, sub-directories allowed and created)
// and returns its size and SHA-256.
//
//   - Names that are empty, ".", absolute or that climb out of Dir ("..") fail with ErrUnsafeName
//     before anything is written.
//   - More than MaxBytes of content fails with ErrTooLarge. Never read more than MaxBytes+1 bytes from r.
//   - The write is atomic: write to a temporary file in the destination directory, then rename.
//     On any failure no new file, no partial file and no temporary file remains, and an existing
//     file with that name keeps its old content.
//   - Hash while copying; do not read the content twice or hold it in memory.
func (s *Store) Save(name string, r io.Reader) (Info, error) {
	panic("TODO")
}

// Open opens a stored file for reading. Unsafe names fail with ErrUnsafeName; a missing file
// returns an error for which errors.Is(err, os.ErrNotExist) is true.
func (s *Store) Open(name string) (*os.File, error) {
	panic("TODO")
}
