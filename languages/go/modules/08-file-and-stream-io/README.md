# 08. File and Stream I/O in Go

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 04

## Learning Outcomes
After this module you can:
- [ ] Read and write files whole (`os.ReadFile`) and as streams (`bufio`, `io.Copy`) and pick the right one by size.
- [ ] Compose `io.Reader`/`io.Writer` (`TeeReader`, `MultiWriter`, `LimitReader`, `Pipe`) to hash, count, limit and transform data in one pass.
- [ ] Work with paths (`path/filepath`), directories, temp files and `io/fs`, including keeping untrusted names inside a directory.
- [ ] Compress (gzip), encode (base64, hex) and hash (SHA-256) data, closing every layer in the right order.
- [ ] Embed static assets in the binary with `embed`.
- [ ] Write files atomically and handle `Close`/`Flush` errors.

## 1. Concept Overview
Most I/O is "bytes flowing from a source to a sink". Treating data as a **stream** keeps memory constant whatever the size (a 5 GiB upload costs the same as a 5 KiB one) and lets you stack behaviours: limit -> decompress -> parse, or copy -> hash -> store. Files, sockets, HTTP bodies, in-memory buffers and compressors all share the same two tiny interfaces, so code written against them works with all of them (and is trivial to test with `strings.NewReader`).

```
source (file / request body / gzip) --Read--> [ limit | tee | decode ] --Write--> sink (file / hash / response)
```

## 2. How Go Approaches It

| Package | Use |
|---------|-----|
| `io` | `Reader`, `Writer`, `Copy`, `TeeReader`, `MultiWriter`, `LimitReader`, `Pipe`, `ReadAll` (small data only) |
| `os` | `Open`, `Create`, `ReadFile`/`WriteFile`, `CreateTemp`/`MkdirTemp`, `Rename`, `OpenRoot` (traversal-safe handle, Go 1.24+) |
| `bufio` | `Scanner` (lines/words; 64 KiB token limit), `Reader`/`Writer` (buffering; `Flush`!) |
| `path/filepath` | OS paths: `Join`, `Clean`, `Rel`, `IsLocal`. (`path` is for slash-only paths such as URLs) |
| `io/fs`, `embed` | `fs.FS` read-only file-system abstraction: `os.DirFS`, `embed.FS`, `testing/fstest.MapFS` |
| `compress/gzip`, `encoding/base64`, `encoding/hex`, `crypto/sha256` | wrappers that implement `io.Reader`/`io.Writer`/`hash.Hash` |

Idiomatic: accept `io.Reader`/`io.Writer` parameters (not file names) in your logic; `defer f.Close()` right after opening; return errors, never ignore `Close` on written files. Non-idiomatic: `os.ReadFile` on user-sized input, `ioutil.*` (deprecated), building paths with `+`. Standard library only; no third-party packages needed.

## 3. Guided Examples
Run all commands from `languages/go/`. Examples use temp directories and clean up after themselves.

### 3.1 Whole files, buffered writes, line scanning
- **Goal:** the three everyday patterns and the `Close`/`Flush` discipline.
- **Run:** `go run ./modules/08-file-and-stream-io/examples/01_read_write_files`
- **Code:** [examples/01_read_write_files/main.go](examples/01_read_write_files/main.go)
```go
defer func() {
	if cerr := f.Close(); err == nil { err = cerr }
}()
w := bufio.NewWriter(f)
...
return w.Flush()
```
- **Walkthrough:** `WriteFile`/`ReadFile` for small files. For bigger data, a `bufio.Writer` batches syscalls but swallows nothing until `Flush`; `Close` on a *written* file may report a failed write-back, so it is folded into the named return. `Scanner.Err()` must be checked after the loop.
- **Expected output:**
  ```
  ReadFile: "Write docs\nFix bug\n"
  line 1: first
  line 2: second
  line 3: third
  missing file is ErrNotExist: true
  ```

### 3.2 Readers and writers compose
- **Goal:** observe a stream without buffering it.
- **Run:** `go run ./modules/08-file-and-stream-io/examples/02_readers_writers`
- **Code:** [examples/02_readers_writers/main.go](examples/02_readers_writers/main.go)
```go
n, err := io.Copy(io.MultiWriter(h, cw), strings.NewReader(text)) // hash + count in one pass
io.ReadAll(io.LimitReader(src, 15))                               // cap an untrusted source
```
- **Walkthrough:** `io.Copy` moves bytes in 32 KiB chunks; `MultiWriter` fans out, `TeeReader` mirrors what a consumer reads, `SectionReader` slices a `ReaderAt`, `io.Pipe` links a producer goroutine to a consumer.
- **Expected output:**
  ```
  copied: 43 counted: 43 err: <nil>
  sha256: d7a8fbb307d7809469ca9abcb0082e4f8d5651e46d3cdb762d02d0bf37c9e592
  head: "The quick" tee: "The quick" err: <nil>
  limited: "The quick brown"
  section: "quick"
  pipe: chunk 1;chunk 2;chunk 3;
  ```

### 3.3 gzip, base64, hex, SHA-256
- **Goal:** transform data with stream wrappers and spot truncation.
- **Run:** `go run ./modules/08-file-and-stream-io/examples/03_gzip_hash_encode`
- **Code:** [examples/03_gzip_hash_encode/main.go](examples/03_gzip_hash_encode/main.go)
- **Walkthrough:** `gzip.NewWriter(&buf)` wraps a sink; **`Close` writes the trailer** (without it the output is corrupt). A truncated archive surfaces as `unexpected EOF` while reading. Encoders (`base64.NewEncoder`) also need `Close` to flush a final partial block. Base64/hex are encodings, not encryption.
- **Expected output:**
  ```
  original bytes: 3400 | smaller after gzip: true
  round trip equal: true
  truncated stream: unexpected EOF
  base64 std: dXNlcjpwYTU1d29yZA==
  base64 url: -__-
  hex: 75736572
  sha256(hello): 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  encoder: c3RyZWFtIG1l
  ```

### 3.4 Paths, traversal safety, `fs.FS` and `embed`
- **Goal:** keep untrusted names inside a directory and treat embedded and real files uniformly.
- **Run:** `go run ./modules/08-file-and-stream-io/examples/04_paths_and_fs`
- **Code:** [examples/04_paths_and_fs/main.go](examples/04_paths_and_fs/main.go) and [assets/](examples/04_paths_and_fs/assets)
```go
func safeJoin(root, name string) (string, error) {
	if !filepath.IsLocal(name) { return "", fmt.Errorf("unsafe path %q", name) }
	return filepath.Join(root, name), nil
}
//go:embed assets
var assets embed.FS
```
- **Walkthrough:** `filepath.Join` cleans `..` and will happily produce `/etc/passwd`; `filepath.IsLocal` rejects absolute and escaping names. A symlink inside the directory can still point out, which only `os.OpenRoot` blocks. `list(fsys fs.FS)` walks `embed.FS` and `os.DirFS` with the same code.
- **Expected output:**
  ```
  data/exports/a.json
  export.tar.gz .gz /var/data
  rel: uploads/a.txt
  naive:  /etc/passwd
  safe: report.pdf         -> "/srv/uploads/report.pdf" err=<nil>
  safe: a/b.txt            -> "/srv/uploads/a/b.txt" err=<nil>
  safe: ../../etc/passwd   -> "" err=unsafe path "../../etc/passwd"
  safe: /etc/passwd        -> "" err=unsafe path "/etc/passwd"
  safe: a/../../b          -> "" err=unsafe path "a/../../b"
  embedded files:
    assets/templates/task.txt (17 bytes)
    assets/welcome.txt (29 bytes)
  directory files:
    sub/note.txt (5 bytes)
  root.Open inside: ok
  root.Open ../outside rejected: true
  root.Open symlink escape rejected: true
  embedded miss is ErrNotExist: true
  ```
  (Paths are shown for Linux/macOS; Windows prints backslashes.)

### 3.5 Export tasks as gzip NDJSON, atomically
- **Goal:** the full pipeline: iterator -> JSON lines -> gzip -> temp file -> rename; and read it back streaming.
- **Run:** `go run ./modules/08-file-and-stream-io/examples/05_task_export`
- **Code:** [examples/05_task_export/main.go](examples/05_task_export/main.go)
```go
tmp, _ := os.CreateTemp(filepath.Dir(path), ".export-*")
zw := gzip.NewWriter(tmp); bw := bufio.NewWriter(zw); enc := json.NewEncoder(bw)
for t := range tasks { enc.Encode(t) }
bw.Flush(); zw.Close(); tmp.Close()   // innermost layer last
os.Rename(tmp.Name(), path)
```
- **Walkthrough:** the temp file lives in the target directory so `Rename` is atomic: readers see the old file or the complete new one. Layers are finished from the outside in. Reading uses `json.Decoder` on the gzip reader, so a 1 GB export needs the same memory as a 1 KB one; a truncated file fails instead of looking complete.
- **Expected output:**
  ```
  exported: 1000
  in directory: tasks.ndjson.gz
  gzip magic: 1f8b
  imported: total=1000 done=333 err=<nil>
  truncated file: true
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `os.ReadFile` / `io.ReadAll` on user input | Convenient; reads everything into memory | `io.Copy` to the destination, bound with `io.LimitReader`/`http.MaxBytesReader` |
| `bufio.Scanner` fails on long lines (`token too long`) | Default 64 KiB token limit; the error only shows via `sc.Err()` | `sc.Buffer(...)`, or use `bufio.Reader.ReadBytes`/`json.Decoder` |
| Ignoring `f.Close()` on written files | `defer f.Close()` discards the error, but write-back errors can surface there | Named return + deferred close that sets `err` (3.1) |
| Forgetting `bufio.Writer.Flush()` / `gzip.Writer.Close()` | Data sits in a buffer; output silently truncated | Flush/Close every layer, outermost first |
| `filepath.Join(root, userName)` is "safe" | `Join` cleans `..` instead of rejecting it | `filepath.IsLocal`, and `os.OpenRoot` for symlink safety |
| Writing straight to the final path | Crash or error leaves a half-written file | `CreateTemp` in the same directory + `Rename` |
| `os.Rename` across file systems / `os.CreateTemp("", ...)` in `/tmp` | Temp dir may be another mount: rename fails | Create the temp file next to the destination |
| `defer f.Close()` in a loop | Deferred calls run when the *function* returns: file descriptors pile up | Move the body into a function, or close explicitly |
| `path` vs `path/filepath` | Both have `Join`; the first always uses `/` | `filepath` for disk, `path` for URLs and `fs.FS` names |
| Treating base64/hex as protection | Both are reversible encodings | Use real crypto (module 11) |

## 5. Coming From Other Languages
- **Java `InputStream`/`BufferedReader`, C# `Stream`:** `io.Reader` + `bufio` play both roles; there is one tiny method per interface and decorators are plain functions. No `using`/try-with-resources: use `defer`.
- **Python `with open(...)`:** `defer f.Close()` right after the error check; no context manager, and no automatic flush for `bufio.Writer`.
- **Node streams:** pull-based (`Read` blocks) instead of event/backpressure-based; `io.Pipe` plus goroutines gives the pipeline feel.
- **Rust `Read`/`Write` traits:** nearly identical shape; Go closes on `defer` rather than `Drop`, so a forgotten `Close` is a leak, not a compile error.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | `wordcount.Count` | Easy | streaming a `Reader` in constant memory | `go test ./modules/08-file-and-stream-io/exercises/ex1_wordcount/` |
| 2 | Safe upload store | Medium | traversal safety, `MultiWriter` hash, size limit, atomic write | `go test ./modules/08-file-and-stream-io/exercises/ex2_safe_store/` |
| 3 | gzip NDJSON stream | Stretch | layered writers, `iter.Seq`, truncation and error handling | `go test ./modules/08-file-and-stream-io/exercises/ex3_ndjson_gzip/` |

**Exercise 1: `wordcount.Count`**
- **Task:** implement `Count(io.Reader)` in [exercises/ex1_wordcount/wordcount.go](exercises/ex1_wordcount/wordcount.go).
- **Acceptance:** correct for the table cases, for readers that return one byte at a time or data together with `io.EOF`, for read errors, and for a 45 MB single line while allocating under 8 MiB.
- <details><summary>Hint</summary>Read into a fixed buffer in a loop and keep a tiny state machine (`inWord`, last byte) across chunks; a word can span two reads.</details>

**Exercise 2: Safe store**
- **Task:** implement `Store.Save` and `Store.Open` in [exercises/ex2_safe_store/store.go](exercises/ex2_safe_store/store.go).
- **Acceptance:** unsafe names give `ErrUnsafeName` and write nothing; size and SHA-256 are right; more than `MaxBytes` gives `ErrTooLarge` without reading the rest; failures leave neither temp files nor a clobbered old file.
- <details><summary>Hint</summary>`io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(r, max+1))`; if you copied more than `max` bytes, it was too large. Clean up in a deferred function that looks at the returned error.</details>

**Exercise 3: gzip NDJSON**
- **Task:** implement `Write` and `Read` in [exercises/ex3_ndjson_gzip/ndjson.go](exercises/ex3_ndjson_gzip/ndjson.go).
- **Acceptance:** round trip; complete gzip stream; writer errors (also from `Close`) returned; blank lines skipped; 1 MiB lines; `*LineError` with line number; truncated/non-gzip input reported; reading stops when the consumer breaks.
- <details><summary>Hint</summary>`bufio.Reader.ReadBytes('\n')` has no line limit. Treat `io.EOF` as normal end but any other error (including `io.ErrUnexpectedEOF`) as failure, even when it arrives together with partial data.</details>

## 7. Recap
- Code against `io.Reader`/`io.Writer`; stream by default, `ReadAll` only for known-small data.
- Layer wrappers (limit, tee, gzip, hash) and finish them from the outside in: `Flush`, then `Close`.
- Check `Close` on written files, `Scanner.Err()`, and `ErrUnexpectedEOF`.
- Never trust a file name: `filepath.IsLocal` (and `os.OpenRoot`); write via temp file + `Rename`.
- Use `fs.FS` for read-only trees so code works on disk, embedded and in-memory.

## 8. Self-check Questions
1. Why is it safe to hash a 10 GB upload with `io.Copy(io.MultiWriter(file, hash), body)` but not with `io.ReadAll`?
2. What exactly is lost if you forget `Flush()` on a `bufio.Writer` over a file, and why does `defer f.Close()` not rescue it?
3. `filepath.Join("/srv/uploads", "../../etc/passwd")` returns `/etc/passwd`. Which function should have run first, and what attack remains even after that?
4. Why must the temp file for an atomic write live in the destination directory?

## 9. Further Reading
- [`io` package](https://pkg.go.dev/io) and [`io/fs`](https://pkg.go.dev/io/fs)
- Go blog: [Traversal-resistant file APIs (`os.Root`)](https://go.dev/blog/osroot)
- [`embed` package](https://pkg.go.dev/embed), [`compress/gzip`](https://pkg.go.dev/compress/gzip)

---
**Next:** [09 Configuration and Environment](../09-configuration/README.md)
