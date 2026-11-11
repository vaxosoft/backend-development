// go:embed compiles files into the binary, so the deployable artifact is a single file.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"text/template"
)

// The pattern is relative to this source file and must not start with "." or "..".
//
//go:embed assets
var assets embed.FS

// A single file can be embedded straight into a string or []byte.
//
//go:embed assets/seed.json
var seedJSON []byte

type Task struct {
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
}

func main() {
	err := fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fmt.Println("embedded:", path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	var tasks []Task
	if err := json.Unmarshal(seedJSON, &tasks); err != nil {
		panic(err)
	}
	fmt.Println("seed tasks:", len(tasks), "first:", tasks[0].Title)

	// fs.Sub re-roots the FS, which is what http.FileServerFS wants for static files.
	static, err := fs.Sub(assets, "assets/static")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(static, "index.html")
	if err != nil {
		panic(err)
	}
	fmt.Printf("index.html: %d lines\n", bytes.Count(index, []byte("\n")))

	tmpl := template.Must(template.ParseFS(assets, "assets/templates/*.tmpl"))
	if err := tmpl.ExecuteTemplate(os.Stdout, "banner.txt.tmpl", map[string]any{"Version": "dev", "Tasks": tasks}); err != nil {
		panic(err)
	}

	_, err = assets.ReadFile("assets/missing.txt")
	fmt.Println("missing file is a normal fs error:", err)
}
