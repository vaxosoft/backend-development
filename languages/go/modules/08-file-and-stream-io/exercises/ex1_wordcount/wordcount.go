// Package wordcount is exercise 1: count lines, words and bytes of a stream in constant memory.
package wordcount

import "io"

// Stats are the results of Count.
type Stats struct {
	Lines int
	Words int
	Bytes int
}

// Count reads r to the end and reports:
//   - Bytes: the total number of bytes read;
//   - Lines: the number of '\n' bytes, plus one more if the data does not end with '\n' (and is not empty);
//   - Words: the number of maximal runs of non-whitespace bytes (whitespace: space, \t, \n, \v, \f, \r).
//
// It must work for inputs of any size and any line length without holding the input in memory,
// and must return a non-EOF read error to the caller.
func Count(r io.Reader) (Stats, error) {
	panic("TODO")
}
