// Package wordcount is exercise 1: count lines, words and bytes of a stream in constant memory.
package wordcount

import (
	"errors"
	"io"
)

// Stats are the results of Count.
type Stats struct {
	Lines int
	Words int
	Bytes int
}

func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// Count reads r to the end and reports:
//   - Bytes: the total number of bytes read;
//   - Lines: the number of '\n' bytes, plus one more if the data does not end with '\n' (and is not empty);
//   - Words: the number of maximal runs of non-whitespace bytes (whitespace: space, \t, \n, \v, \f, \r).
//
// It must work for inputs of any size and any line length without holding the input in memory,
// and must return a non-EOF read error to the caller.
func Count(r io.Reader) (Stats, error) {
	var s Stats
	buf := make([]byte, 32*1024)
	inWord := false
	var last byte
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			if b == '\n' {
				s.Lines++
			}
			if isSpace(b) {
				inWord = false
			} else if !inWord {
				inWord = true
				s.Words++
			}
		}
		if n > 0 {
			s.Bytes += n
			last = buf[n-1]
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return s, err
		}
	}
	if s.Bytes > 0 && last != '\n' {
		s.Lines++
	}
	return s, nil
}
