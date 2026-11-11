# 08. File and Stream I/O in Rust

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 04

## Learning Outcomes
After this module you can:
- [ ] Read and write files with `std::fs`, buffered readers/writers and `Path`/`PathBuf`
- [ ] Write code against `Read`/`Write` so it works on files, memory and sockets alike
- [ ] Stream large data in constant memory (hashing, copying, compressing) instead of loading it
- [ ] Compress (gzip) and encode (base64, hex) data, and compute SHA-256 digests
- [ ] Store untrusted uploads safely: sanitized names, size limits, atomic writes, no path traversal
- [ ] Embed small assets in the binary with `include_str!`

## 1. Concept Overview
Almost every I/O task is "move bytes from a source to a sink". A **stream** abstraction (reader, writer) lets you process data in small chunks, so a 5 GiB upload costs the same memory as a 5 KiB one. Layers compose: `file -> buffer -> gzip -> hash`. The traps are all at the edges: forgetting that buffers must be flushed, trusting a client-supplied file name, and leaving half-written files behind when something fails.

```
source (File, socket, &[u8])  ->  [adapters: BufReader, take, GzDecoder]  ->  your loop  ->  [BufWriter, GzEncoder]  ->  sink
```

## 2. How Rust Approaches It

| Item | Use |
|------|-----|
| `std::fs` | whole-file helpers (`read_to_string`, `write`), `File`, `OpenOptions`, `read_dir`, `create_dir_all` |
| `std::io::{Read, Write, BufRead}` | the stream traits; `Vec<u8>`, `&[u8]` and `Cursor` implement them, so tests need no files |
| `BufReader` / `BufWriter` | batch many small reads/writes into few syscalls |
| `io::copy`, `Read::take`, `chain` | move, limit and concatenate streams |
| `std::path::{Path, PathBuf}` | typed, OS-aware paths (not `String`) |
| `tempfile` | `tempdir()`, `NamedTempFile` (+ `persist` for atomic rename) |
| `flate2` | gzip as `GzEncoder<W: Write>` / `GzDecoder<R: Read>` wrappers |
| `sha2`, `hex`, `base64` | digests and text encodings; `Digest::update` makes hashing incremental |
| `tokio::fs`, `tokio::io` | async equivalents (`AsyncReadExt`); file I/O uses a thread pool underneath |
| `include_str!`, `include_bytes!` | embed a file at compile time |

- **Idiomatic:** take `impl Read`/`impl Write` (or `R: Read`) instead of `&Path` when the logic does not care where bytes live; return `io::Result`; use `?`.
- **Avoid:** `fs::read` of unbounded input, `.unwrap()` on I/O, and building paths with `format!("{dir}/{name}")`.
- Blocking `std::fs` inside an async handler blocks a runtime thread: use `tokio::fs` or `spawn_blocking` (module 06).

## 3. Guided Examples

### 3.1 Files, paths and buffered lines
- **Goal:** the everyday `std::fs` / `Path` / `BufRead` toolkit in a temp directory.
- **Run:** `cargo run -p m08-file-and-stream-io --example 01_files_and_paths`
- **Code:** [examples/01_files_and_paths.rs](examples/01_files_and_paths.rs)
- **Walkthrough:** `tempfile::tempdir()` is deleted on drop. `fs::write`/`read_to_string` are fine for small files. `BufReader::new(File::open(..)?).lines()` reads one line at a time. A `BufWriter` flushes on drop but **ignores** errors then, so the example calls `out.flush()?`. `read_dir` order is unspecified, hence the sort.
- **Expected output:**
  ```
  file_name="today.txt" extension="txt"
  relative: notes/today.txt
  size: 21 bytes
  content: write docs
  review PR
   1: write docs
   2: review PR
   3: ship it
  dir  notes
  file report.txt
  missing.txt -> NotFound
  ```

### 3.2 Readers and writers are the abstraction
- **Goal:** write generic stream code and wrap streams with adapters.
- **Run:** `cargo run -p m08-file-and-stream-io --example 02_readers_and_writers`
- **Code:** [examples/02_readers_and_writers.rs](examples/02_readers_and_writers.rs)
- **Walkthrough:**
  ```rust
  fn shout<R: BufRead, W: Write>(input: R, mut output: W) -> io::Result<()>
  ```
  runs unchanged on `&[u8]`, `Cursor`, files and stdout. `CountingWriter<W>` wraps any writer and counts what the inner writer **returned** from `write` (it may accept less than offered). `take(4)` caps a reader (the building block of "max upload size"), `chain` joins two, and `io::copy` streams in chunks. `read_exact` fails with `UnexpectedEof` instead of returning a short read.
- **Expected output:**
  ```
  23 bytes: "WRITE DOCS!\nREVIEW PR!\n"
  take(4): 0123
  copied 12 bytes: "header\nbody\n"
  DONE!
  read_exact on 2 bytes -> UnexpectedEof
  ```

### 3.3 Streaming a large file through SHA-256
- **Goal:** constant memory, sync and async.
- **Run:** `cargo run -p m08-file-and-stream-io --example 03_streaming_sha256`
- **Code:** [examples/03_streaming_sha256.rs](examples/03_streaming_sha256.rs)
- **Walkthrough:** an 8 MiB file is written in 64 KiB chunks while being hashed, then re-read with an 8 KiB buffer: `read` returns `n` bytes (possibly fewer than the buffer), `0` means EOF, and only `buffer[..n]` is valid. The async version uses `tokio::fs::File` and `AsyncReadExt::read` with the same loop. All three digests agree.
- **Expected output:**
  ```
  wrote 8388608 bytes, sha256 cad65a1f46032f6d4ecf9086f0093778849d5ece5a6722471600fd02c80ee025
  sync read:  cad65a1f46032f6d4ecf9086f0093778849d5ece5a6722471600fd02c80ee025
  async read: cad65a1f46032f6d4ecf9086f0093778849d5ece5a6722471600fd02c80ee025
  all equal: true
  ```

### 3.4 Compression and encoding
- **Goal:** gzip as stream wrappers; base64 and hex.
- **Run:** `cargo run -p m08-file-and-stream-io --example 04_compression_and_encoding`
- **Code:** [examples/04_compression_and_encoding.rs](examples/04_compression_and_encoding.rs)
- **Walkthrough:** `GzEncoder::new(Vec::new(), ..)` is a `Write`; call `finish()` to write the trailer and get the inner writer back (dropping it silently may lose the trailer). `GzDecoder::new(reader)` is a `Read`, so it composes with `read_to_string`. Corrupt input is an `Err`, never a panic. `STANDARD` base64 uses `+` and `/`; `URL_SAFE_NO_PAD` uses `-` and `_` (use it for tokens in URLs).
- **Expected output:**
  ```
  gzip magic: 1f8b
  smaller than the input: true
  round trip equal: true
  invalid gzip -> InvalidInput
  hex:        48656c6c6f2c205461736b20547261636b65723f21
  base64:     SGVsbG8sIFRhc2sgVHJhY2tlcj8h
  standard:   +//+
  url safe:   -__-
  decoded: Hello
  bad base64 -> Invalid symbol 42, offset 0.
  bad hex    -> Odd number of digits
  ```

### 3.5 Task Tracker export: embedded seed, gzip JSON Lines, atomic write
- **Goal:** the pieces in the domain: `include_str!` seed data, streamed export, atomic replace.
- **Run:** `cargo run -p m08-file-and-stream-io --example 05_task_export`
- **Code:** [examples/05_task_export.rs](examples/05_task_export.rs), asset [examples/assets/seed_tasks.jsonl](examples/assets/seed_tasks.jsonl)
- **Walkthrough:** `export` writes `serde_json::to_writer` lines through `GzEncoder<BufWriter<File>>` into a `NamedTempFile` in the **same directory** as the target, then `persist` renames it over the target. Readers never see a half-written file and a crash leaves the old one intact. `read_tasks` returns a lazy iterator: `BufReader<GzDecoder<File>>::lines()` parses one record at a time.
- **Expected output:**
  ```
  embedded seed: 4 tasks, 539 bytes
  exported to tasks.jsonl.gz
  by status: {"done": 1, "in_progress": 1, "todo": 2}
  directory: ["tasks.jsonl.gz"]
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Output file is truncated or empty | `BufWriter`/`GzEncoder` keep data in memory; their `Drop` flushes but **swallows errors**, and a leaked or `mem::forget`-ed writer never flushes | Call `flush()?` / `finish()?` explicitly, and unwrap the layers with `into_inner()` |
| `fs::read_to_string` / `read_to_end` on upload or log data | Convenient, and fine in tests | Stream with a fixed buffer, or cap with `reader.take(limit + 1)` |
| Assuming one `read`/`write` moves everything | They may transfer fewer bytes than requested | Use `read_exact`, `write_all` or `io::copy`, and honour the returned count (exercise 1) |
| `dir.join(client_name)` | `Path::join` with an absolute component **replaces** the base; `..` walks out | Never use client names as paths: generate ids, sanitize the display name (exercise 2) |
| Writing straight to the final path | A failure leaves a half-written file visible | `NamedTempFile::new_in(same_dir)` then `persist(target)` (rename is only atomic within one filesystem) |
| Using `String` for paths | Not every path is valid UTF-8 and separators differ | `Path`/`PathBuf`; `to_string_lossy()` only for display |
| `std::fs`/`std::io` in an `async fn` handler | Blocks a runtime worker thread | `tokio::fs`, or `tokio::task::spawn_blocking` |
| Relying on `read_dir` order | The OS decides | Collect and sort |

## 5. Coming From Other Languages
- **Go:** `io.Reader`/`io.Writer` map directly to `Read`/`Write`; `io.Copy` is `io::copy`; `defer f.Close()` is unnecessary (files close on drop) but `Close` errors on writers are surfaced by explicit `flush`/`sync_all`.
- **C++:** RAII files and `std::ostream` buffering; Rust's `BufWriter` flush-on-drop has the same "destructor cannot report errors" caveat.
- **Python:** `with open(...)` is just scope; `shutil.copyfileobj` is `io::copy`; `tempfile` has the same name and idea.
- **TypeScript:** Node streams/`pipeline` become plain `Read`/`Write` composition (sync) or `tokio::io` (async); no event emitters.
- **Java/C#:** `InputStream`/`Stream` decorators (`GZIPInputStream`) are Rust's wrapper types; try-with-resources/`using` is automatic drop.

## 6. Exercises
All tests use temp directories and in-memory streams only.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Counting writer | Easy | implementing `Write`, partial writes, adapters | `cargo test -p m08-ex1-counting-writer` |
| 2 | Safe upload store | Medium | streaming, size limits, hashing, sanitizing, cleanup on failure | `cargo test -p m08-ex2-safe-upload-store` |
| 3 | Gzip pack with checksum | Stretch | gzip streams, atomic output, verify-before-publish | `cargo test -p m08-ex3-gzip-pack` |

**Exercise 1: `CountingWriter`** ([starter](exercises/ex1-counting-writer/src/lib.rs), [tests](exercises/ex1-counting-writer/tests/counting_writer.rs))
- **Task:** wrap a `Write`, count the bytes the inner writer accepted, forward `flush`, implement `bytes_written` and `into_inner`.
- **Acceptance:** counts equal accepted bytes even when the inner writer takes 3 bytes per call; errors are not counted; composes with `BufWriter`.
<details><summary>Hint</summary>Count the <code>Ok(n)</code> that the inner <code>write</code> returns, not <code>buf.len()</code>.</details>

**Exercise 2: `store_upload`** ([starter](exercises/ex2-safe-upload-store/src/lib.rs), [tests](exercises/ex2-safe-upload-store/tests/store.rs))
- **Task:** `sanitize_filename` (keep last path component, drop control characters) and `store_upload` (validate the id, stream to a temp file in the root, hash while copying, reject bodies over the limit, publish by rename).
- **Acceptance:** `../../etc/passwd` can never leave the root; exactly the limit passes, one byte more fails with nothing left on disk; an endless reader is cut off; a read error leaves no files.
<details><summary>Hint</summary><code>reader.take(max + 1)</code> lets you detect "too large" after reading at most one extra byte.</details>

**Exercise 3: `pack` / `unpack_verified`** ([starter](exercises/ex3-gzip-pack/src/lib.rs), [tests](exercises/ex3-gzip-pack/tests/pack.rs))
- **Task:** stream data through `GzEncoder` into a temp file while hashing the raw bytes; `persist` it atomically. `unpack_verified` decompresses to a temp file and only publishes it if the SHA-256 matches.
- **Acceptance:** round trip; a failing source keeps the previous destination file; checksum mismatch and truncated archives create nothing.
<details><summary>Hint</summary>Write a small <code>copy_hashing(reader, writer)</code> helper and use it for both directions; call <code>encoder.finish()</code> before <code>persist</code>.</details>

Reference solutions: [solutions/](solutions/) (overlay onto `exercises/`).

## 7. Recap
- Program against `Read`/`Write`; adapters (`BufReader`, `take`, `GzDecoder`) stack like decorators.
- Stream in fixed-size chunks; never `read_to_end` untrusted input.
- Flush and finish explicitly; do not rely on `Drop` for error reporting.
- Write to a temp file in the target directory, then rename, so failures leave no trace.
- Treat every client-supplied name as hostile: generate ids, sanitize what you display.

## 8. Self-check Questions
1. Why can `write(buf)` return less than `buf.len()`, and which std method hides that for you?
2. What goes wrong if a `BufWriter<File>` is dropped while the disk is full?
3. Why must the temp file be created in the same directory as the final file?
4. `base.join("/etc/passwd")` gives what path, and why does that matter for uploads?
5. How would you cap an upload at 5 MiB without knowing its `Content-Length`?

## 9. Further Reading
- [`std::io` module docs](https://doc.rust-lang.org/std/io/index.html) and [`std::path`](https://doc.rust-lang.org/std/path/index.html)
- [`tempfile`](https://docs.rs/tempfile), [`flate2`](https://docs.rs/flate2), [`sha2`](https://docs.rs/sha2)
- OWASP, "Path Traversal"

---
**Next:** [09 Configuration](../09-configuration/README.md)
