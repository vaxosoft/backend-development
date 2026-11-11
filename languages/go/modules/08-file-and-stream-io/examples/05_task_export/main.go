// Example 05: stream Task Tracker tasks to a gzip-compressed NDJSON file (atomically) and back.
package main

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"os"
	"path/filepath"
)

type Task struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
}

func generate(n int) iter.Seq[Task] {
	return func(yield func(Task) bool) {
		for i := 1; i <= n; i++ {
			status := "todo"
			if i%3 == 0 {
				status = "done"
			}
			if !yield(Task{ID: i, Title: fmt.Sprintf("Task number %d", i), Status: status, Tags: []string{"demo"}}) {
				return
			}
		}
	}
}

// export writes to a temp file in the same directory and renames it into place, so readers never see a half-written file.
func export(path string, tasks iter.Seq[Task]) (n int, err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".export-*")
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	zw := gzip.NewWriter(tmp)
	bw := bufio.NewWriter(zw)
	enc := json.NewEncoder(bw) // Encode ends every value with "\n": that is NDJSON
	for t := range tasks {
		if err := enc.Encode(t); err != nil {
			return n, err
		}
		n++
	}
	// Close from the outermost layer inwards: buffer -> gzip -> file.
	if err := bw.Flush(); err != nil {
		return n, err
	}
	if err := zw.Close(); err != nil {
		return n, err
	}
	if err := tmp.Close(); err != nil {
		return n, err
	}
	return n, os.Rename(tmp.Name(), path)
}

func importDone(path string) (total, done int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return 0, 0, err
	}
	defer zr.Close()

	dec := json.NewDecoder(zr)
	for {
		var t Task
		if err := dec.Decode(&t); err == io.EOF {
			return total, done, nil
		} else if err != nil {
			return total, done, fmt.Errorf("record %d: %w", total+1, err)
		}
		total++
		if t.Status == "done" {
			done++
		}
	}
}

func main() {
	dir, err := os.MkdirTemp("", "export-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "tasks.ndjson.gz")

	n, err := export(path, generate(1000))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("exported:", n)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		fmt.Println("in directory:", e.Name())
	}

	f, _ := os.Open(path)
	magic := make([]byte, 2)
	io.ReadFull(f, magic)
	f.Close()
	fmt.Println("gzip magic:", hex.EncodeToString(magic))

	total, done, err := importDone(path)
	fmt.Printf("imported: total=%d done=%d err=%v\n", total, done, err)

	// Cut the file short: the reader reports it instead of returning partial data as complete.
	data, _ := os.ReadFile(path)
	cut := filepath.Join(dir, "cut.ndjson.gz")
	os.WriteFile(cut, data[:len(data)/2], 0o644)
	_, _, err = importDone(cut)
	fmt.Println("truncated file:", err != nil)
}
