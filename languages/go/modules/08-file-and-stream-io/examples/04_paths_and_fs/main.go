// Example 04: path handling, traversal-safe joins, os.Root, and one walk over embed.FS and os.DirFS.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

//go:embed assets
var assets embed.FS

// safeJoin resolves an untrusted relative name below root, or fails.
func safeJoin(root, name string) (string, error) {
	if !filepath.IsLocal(name) {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	return filepath.Join(root, name), nil
}

// list prints every file in any fs.FS: the code does not care where the bytes live.
func list(fsys fs.FS) {
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			data, err := fs.ReadFile(fsys, p)
			if err != nil {
				return err
			}
			fmt.Printf("  %s (%d bytes)\n", p, len(data))
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	fmt.Println(filepath.Join("data", "tasks", "../exports", "a.json"))
	fmt.Println(filepath.Base("/var/data/export.tar.gz"), filepath.Ext("export.tar.gz"), filepath.Dir("/var/data/export.tar.gz"))
	rel, _ := filepath.Rel("/srv/app", "/srv/app/uploads/a.txt")
	fmt.Println("rel:", rel)

	// Join happily walks out of the directory.
	fmt.Println("naive: ", filepath.Join("/srv/uploads", "../../etc/passwd"))
	for _, name := range []string{"report.pdf", "a/b.txt", "../../etc/passwd", "/etc/passwd", "a/../../b"} {
		p, err := safeJoin("/srv/uploads", name)
		fmt.Printf("safe: %-18s -> %q err=%v\n", name, p, err)
	}

	dir, err := os.MkdirTemp("", "fs-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "note.txt"), []byte("hello"), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("embedded files:")
	list(assets)
	fmt.Println("directory files:")
	list(os.DirFS(dir))

	// A symlink pointing outside the directory defeats string checks, but not os.Root.
	_ = os.Symlink("/etc/hosts", filepath.Join(dir, "sub", "escape"))

	root, err := os.OpenRoot(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer root.Close()
	if f, err := root.Open("sub/note.txt"); err == nil {
		fmt.Println("root.Open inside: ok")
		f.Close()
	}
	_, err = root.Open("../outside")
	fmt.Println("root.Open ../outside rejected:", err != nil)
	_, err = root.Open("sub/escape")
	fmt.Println("root.Open symlink escape rejected:", err != nil)

	_, err = fs.ReadFile(assets, "assets/nope.txt")
	fmt.Println("embedded miss is ErrNotExist:", errors.Is(err, fs.ErrNotExist))
}
