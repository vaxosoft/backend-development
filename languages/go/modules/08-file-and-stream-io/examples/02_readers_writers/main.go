// Example 02: composing io.Reader and io.Writer: Tee, MultiWriter, LimitReader, Pipe.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

func main() {
	const text = "The quick brown fox jumps over the lazy dog"

	// Copy once, observe it twice: hash and count while the bytes flow past.
	h := sha256.New()
	cw := &countingWriter{}
	n, err := io.Copy(io.MultiWriter(h, cw), strings.NewReader(text))
	fmt.Println("copied:", n, "counted:", cw.n, "err:", err)
	fmt.Println("sha256:", hex.EncodeToString(h.Sum(nil)))

	// TeeReader writes everything the consumer reads into a second writer.
	var seen strings.Builder
	head := make([]byte, 9)
	_, err = io.ReadFull(io.TeeReader(strings.NewReader(text), &seen), head)
	fmt.Printf("head: %q tee: %q err: %v\n", head, seen.String(), err)

	// LimitReader caps how much an untrusted source can deliver.
	limited, _ := io.ReadAll(io.LimitReader(strings.NewReader(text), 15))
	fmt.Printf("limited: %q\n", limited)

	// SectionReader gives random access over a ReaderAt.
	sec := io.NewSectionReader(strings.NewReader(text), 4, 5)
	part, _ := io.ReadAll(sec)
	fmt.Printf("section: %q\n", part)

	// Pipe connects a writer goroutine to a reader without buffering everything.
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for i := 1; i <= 3; i++ {
			fmt.Fprintf(pw, "chunk %d;", i)
		}
	}()
	all, _ := io.ReadAll(pr)
	fmt.Printf("pipe: %s\n", all)
}
