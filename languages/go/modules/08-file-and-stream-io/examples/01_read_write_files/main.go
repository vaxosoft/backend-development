// Example 01: whole-file helpers, buffered writing with a checked Close, and line-by-line reading.
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func writeLines(path string, lines []string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	// For written files Close can report a failed flush to disk: do not drop that error.
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	w := bufio.NewWriter(f)
	for _, l := range lines {
		if _, err := fmt.Fprintln(w, l); err != nil {
			return err
		}
	}
	return w.Flush() // buffered bytes are lost without this
}

func main() {
	dir, err := os.MkdirTemp("", "io-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "tasks.txt")

	// Small files: one call each way.
	if err := os.WriteFile(path, []byte("Write docs\nFix bug\n"), 0o644); err != nil {
		log.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ReadFile: %q\n", data)

	// Larger or unknown-size files: stream.
	if err := writeLines(path, []string{"first", "second", "third"}); err != nil {
		log.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close() // read-only: the Close error carries no information

	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		fmt.Printf("line %d: %s\n", n, sc.Text())
	}
	if err := sc.Err(); err != nil { // Scan returning false can also mean an error
		log.Fatal(err)
	}

	_, err = os.ReadFile(filepath.Join(dir, "missing.txt"))
	fmt.Println("missing file is ErrNotExist:", os.IsNotExist(err))
}
