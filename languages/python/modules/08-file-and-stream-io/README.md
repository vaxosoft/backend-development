# 08. File and Stream I/O in Python

> Estimated time: 1.5-2h | Difficulty: Beginner-Intermediate | Prerequisites: 01 (language fundamentals, context managers), 04 (serialization)

## Learning Outcomes
After this module you can:
- [ ] Read and write text and binary files with `pathlib`, naming the encoding explicitly
- [ ] Process data of unknown size in chunks instead of loading it into memory
- [ ] Write functions against file-like objects so they work with files, sockets and in-memory buffers
- [ ] Compress with gzip/zip and encode with base64/hex, and hash data while streaming it
- [ ] Reject path traversal and store uploads under generated ids
- [ ] Write files atomically and read bundled assets with `importlib.resources`

## 1. Concept Overview
Files, sockets, pipes and HTTP bodies share one abstraction: a **stream** you read from or write to. The two rules that matter for servers are *bound your memory* (process a chunk at a time, because the input may be gigabytes or hostile) and *never trust a path* (a user-supplied name must not decide where bytes land).

Around streams sit a few recurring tools: temp files and directories for scratch space, **compression** (gzip for one stream, zip for many named members), **encodings** such as base64 and hex (bytes to safe text, *not* encryption), **hashing** for fingerprints and ETags, and **embedded assets** that ship inside your package.

## 2. How Python Approaches It

| Tool | Use it for |
|------|-----------|
| `pathlib.Path` | paths as objects: `/` to join, `read_text`, `write_bytes`, `glob`, `mkdir(parents=True, exist_ok=True)` |
| `open()` / `Path.open()` | text mode (`"r"`, `"w"`, `"a"`, `"x"`) decodes to `str`; binary mode (`"rb"`, `"wb"`) gives `bytes` |
| `io.BytesIO`, `io.StringIO` | in-memory file objects: ideal for tests and small payloads |
| `shutil.copyfileobj`, `hashlib.file_digest` | chunked copy and hashing of any file object |
| `tempfile.TemporaryDirectory`, `NamedTemporaryFile`, `mkstemp` | scratch space that cleans itself up |
| `gzip`, `zipfile`, `base64`, `bytes.hex()`, `hashlib` | compression, encodings, digests |
| `importlib.resources.files(package)` | read data files that live next to your code |

The idiom is `with` for every resource, `pathlib` for paths and **always `encoding="utf-8"`** in text mode: the default is locale-dependent, so code that works on your laptop breaks on a server with a different locale. For typing, accept `BinaryIO`/`TextIO` or a small `Protocol` (`read(size) -> bytes`) rather than a concrete file class. Only the standard library is used in this module.

## 3. Guided Examples
Run from `languages/python/`.

### 3.1 pathlib, text and bytes
- **Goal:** paths as objects, explicit encodings, open modes.
- **Run:** `python modules/08-file-and-stream-io/examples/01_pathlib_text_binary.py`
- **Code:** [01_pathlib_text_binary.py](examples/01_pathlib_text_binary.py)

```python
notes.write_text("café\nnaïve\n", encoding="utf-8")
raw = notes.read_bytes()  # 13 bytes for 11 characters
raw.decode("latin-1")  # mojibake: the wrong codec
with notes.open("x", encoding="utf-8"):
    ...  # FileExistsError: "x" never overwrites
```

- **Walkthrough:** characters and bytes differ as soon as text is not ASCII. Decoding with the wrong codec does not fail, it silently produces garbage. `"x"` is the safe "create only" mode. A missing file raises `FileNotFoundError`; catch it instead of checking `exists()` first (a race).
- **Expected output:**

  ```
  todo.txt todo .txt notes
  characters: 11 bytes: 13
  decoded with the wrong codec: 'cafÃ©\nnaÃ¯ve\n'
  lines: ['café', 'naïve', 'third line']
  mode 'x' on an existing file: FileExistsError
  ['data.bin', 'notes', 'notes/done.txt', 'notes/todo.txt']
  *.txt: ['done.txt', 'todo.txt']
  size: 4 is_file: True
  raised: FileNotFoundError
  ```

### 3.2 Streaming in chunks
- **Goal:** bounded memory, hashing while reading, file-like objects as the common interface.
- **Run:** `python modules/08-file-and-stream-io/examples/02_streaming_chunks.py`
- **Code:** [02_streaming_chunks.py](examples/02_streaming_chunks.py)

```python
def sha256_of(stream: BinaryIO) -> str:
    digest = hashlib.sha256()
    while chunk := stream.read(CHUNK):  # b"" means end of stream
        digest.update(chunk)
    return digest.hexdigest()


shutil.copyfileobj(reader, writer, length=CHUNK)
```

- **Walkthrough:** the same `sha256_of` works for a real file and for `io.BytesIO`. `hashlib.file_digest` is the stdlib shortcut. Iterating a text file yields one line at a time. After the `with` block the handle is closed and further use raises `ValueError`.
- **Expected output:**

  ```
  sha256 by chunks:   aca1cd027e979588
  hashlib.file_digest: aca1cd027e979588
  copy size: 1048576
  in-memory: 2cf24dba5fb0a30e
  lines in a StringIO: 3
  lines in a file: 3
  closed file: I/O operation on closed file.
  ```

### 3.3 Compression, encodings and hashing
- **Goal:** gzip and zip, base64/hex, digests.
- **Run:** `python modules/08-file-and-stream-io/examples/03_compression_encoding.py`
- **Code:** [03_compression_encoding.py](examples/03_compression_encoding.py)

```python
packed = gzip.compress(payload, mtime=0)
with gzip.open(buffer, "wt", encoding="utf-8") as out:
    ...  # text over gzip
with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as zf:
    ...
base64.urlsafe_b64encode(token)  # "-" and "_" instead of "+" and "/": safe in URLs
```

- **Walkthrough:** gzip is a single stream (use it for `.ndjson.gz`), zip holds named members. Standard and URL-safe base64 differ in two characters. `md5` appears only to say it is acceptable for checksums, never for security (`usedforsecurity=False`).
- **Expected output:**

  ```
  gzip smaller: True | round trip ok: True
  gzip text lines: ['one', 'two']
  zip members: ['tasks.json', 'notes/readme.txt']
  read member: [{"id": 1}]
  hex:       fbffbefa00 True
  base64:    +/+++gA=
  urlsafe:   -_---gA=
  decoded:   True
  sha256:    2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  md5 only for checksums, not security: 5d41402abc4b2a76b9719d911017c592
  ```

### 3.4 Safe paths and bundled assets
- **Goal:** stop path traversal; read a file that ships inside a package.
- **Run:** `python modules/08-file-and-stream-io/examples/04_safe_paths_and_assets.py`
- **Code:** [04_safe_paths_and_assets.py](examples/04_safe_paths_and_assets.py), asset in [m08_assets/](examples/m08_assets/welcome.txt)

```python
root = base.resolve()
candidate = (root / user_path).resolve()  # collapses "..", follows symlinks
if not candidate.is_relative_to(root):
    raise PathTraversalError(...)
resources.files("m08_assets").joinpath("welcome.txt").read_text(encoding="utf-8")
```

- **Walkthrough:** `resolve()` first, then `is_relative_to()`: comparing strings with `startswith` is both wrong (`/uploads2`) and bypassable. An absolute user path replaces the base when joined, and a symlink inside the directory can point outside; both are caught. `sanitize_filename` only produces a display name. Never use it as a path.
- **Expected output:**

  ```
  report.txt           ok -> report.txt
  sub/../report.txt    ok -> report.txt
  ../secret.txt        rejected
  /etc/passwd          rejected
  link                 rejected
  '../../etc/passwd'       -> 'passwd'
  'C:\\temp\\evil.exe'     -> 'evil.exe'
  'report (final).pdf'     -> 'report _final_.pdf'
  '..'                     -> 'file'
  'bad\x00name.txt'        -> 'bad_name.txt'
  Welcome to the Task Tracker!
  ```

### 3.5 Task export: NDJSON, gzip, atomic write
- **Goal:** stream 10 000 tasks to disk without holding them in memory, and never leave a half-written file.
- **Run:** `python modules/08-file-and-stream-io/examples/05_task_export.py`
- **Code:** [05_task_export.py](examples/05_task_export.py)

```python
with atomic_write(path) as raw, gzip.open(raw, "wt", encoding="utf-8") as out:
    for task in tasks:  # a generator: one task in memory at a time
        out.write(json.dumps(task, separators=(",", ":")) + "\n")
```

- **Walkthrough:** `atomic_write` writes to a temp file in the *same directory* and `os.replace`s it onto the target, so readers see the old file or the complete new one. When the generator fails halfway, the temp file is removed and the old export is untouched. Reading back is a generator too.
- **Expected output:**

  ```
  exported: 10000
  by status: {'done': 3333, 'in_progress': 3334, 'todo': 3333}
  sha256 of the uncompressed export: fd9faa5ed95a80e0
  second export failed: database connection lost
  old export untouched: True
  directory contents: ['tasks.ndjson.gz']
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `open(path)` / `read_text()` without `encoding=` | the default is the locale encoding, so behaviour changes between machines | always pass `encoding="utf-8"` (binary mode takes none) |
| `f.read()` on an upload or a big file | it is the shortest code | loop on `read(chunk_size)`, `copyfileobj`, or iterate lines; cap total size |
| `f = open(...)` without `with` | the garbage collector "usually" closes it | `with path.open(...) as f:`; the file is closed even on exceptions |
| `Path(base) / user_input` | `/` happily accepts `../x` and absolute paths (an absolute right side discards the base) | `resolve()` then `is_relative_to(base.resolve())`, or better, store under generated ids |
| `str(path).startswith(str(base))` | looks like a containment check | it matches `/data2` for `/data` and ignores symlinks; use `is_relative_to` |
| Writing straight to the final file | simplest code | write a temp file in the same directory and `os.replace` it |
| `tempfile.mktemp()` or a fixed `/tmp/x` name | predictable names | `TemporaryDirectory`, `NamedTemporaryFile`, `mkstemp` |
| `gzip.open(path)` then iterating lines | default mode is binary (`"rb"`) | open with `"rt", encoding="utf-8"` for text |
| `Path.exists()` then `open()` | check-then-act race | just open it and catch `FileNotFoundError` |
| Thinking base64 hides secrets | it looks scrambled | it is an encoding, not encryption; use real crypto (module 11) |

## 5. Coming From Other Languages
- **Go:** `io.Reader`/`io.Writer` become file-like objects (`read(n)`, `write(b)`); `io.Copy` is `shutil.copyfileobj`; `defer f.Close()` is `with`. Bytes and `str` are different types, so decoding is explicit.
- **Java/C#:** `try-with-resources`/`using` is `with`; `Path`/`File` are `pathlib.Path`; `MemoryStream` is `io.BytesIO`.
- **JavaScript/TypeScript:** Node streams and `fs.promises` have no direct equivalent here: Python file I/O is blocking (use a thread pool or `anyio` from async code), and chunking is a plain `while` loop or generator.
- **Ruby:** `File.open` blocks map to `with`; `Pathname` maps to `pathlib`.

## 6. Exercises
Starters and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/). Tests use `tmp_path` and in-memory streams only.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Upload store](exercises/ex1_safe_paths/m08_upload_store.py) | Easy | generated ids, sanitised display names, no traversal | `pytest modules/08-file-and-stream-io/exercises/ex1_safe_paths` |
| 2 | [Copy and hash](exercises/ex2_copy_and_hash/m08_copy_hash.py) | Medium | chunked copy, hashing, size limit | `pytest modules/08-file-and-stream-io/exercises/ex2_copy_and_hash` |
| 3 | [NDJSON + gzip](exercises/ex3_ndjson_gzip/m08_ndjson.py) | Stretch | atomic write, lazy reader, gzip text mode | `pytest modules/08-file-and-stream-io/exercises/ex3_ndjson_gzip` |

**Exercise 1: `UploadStore`.** *Task:* store bytes at `base_dir/<generated id>`, return a sanitised display name, and make `read`/`delete` accept only well-formed, existing ids. *Acceptance:* all tests in `test_m08_upload_store.py`, including `../../outside.txt` and ids such as `../secret.txt`.
<details><summary>Hint</summary>The client filename never touches the path. Validate ids with a strict regular expression before building a path from them.</details>

**Exercise 2: `copy_and_hash`.** *Task:* copy `src` to `dst` in chunks, return size and SHA-256, and raise `TooLargeError` as soon as the limit is crossed without writing the offending chunk. *Acceptance:* all tests in `test_m08_copy_hash.py` (short reads, an endless source, exactly-at-limit).
<details><summary>Hint</summary><code>while chunk := src.read(chunk_size):</code> ends on an empty read, and a read may return fewer bytes than asked.</details>

**Exercise 3: `write_ndjson_gz` / `read_ndjson_gz`.** *Task:* write records as compact UTF-8 JSON lines inside gzip, atomically; read them back lazily with line-numbered errors. *Acceptance:* all tests in `test_m08_ndjson.py`, including the failure that must leave the previous file and no temp file.
<details><summary>Hint</summary>Use <code>tempfile.mkstemp(dir=path.parent)</code>, wrap the descriptor with <code>os.fdopen</code> and <code>gzip.open(raw, "wt")</code>, and clean up in an <code>except BaseException</code> that re-raises.</details>

## 7. Recap
- Name the encoding, use `with`, use `pathlib`.
- Stream: chunk loops, `copyfileobj`, generators; cap the total size of anything a client sends.
- Write functions against file-like objects so tests can pass `BytesIO`.
- Paths from users are data, not paths: generated ids, or `resolve()` plus `is_relative_to()`.
- Write to a temp file and `os.replace` when a partial file would be harmful.

## 8. Self-check Questions
1. A text file is 11 characters long but 13 bytes on disk. What does that tell you about decoding, and what goes wrong if you pick the wrong codec?
2. Why must the temp file for an atomic write live in the same directory as the target?
3. Why is `os.path.join(base, user_name)` followed by a `startswith(base)` check not enough?
4. What would happen to memory use if `copy_and_hash` called `src.read()` once, and how does the limit protect against an endless source?

## 9. Further Reading
- [pathlib](https://docs.python.org/3/library/pathlib.html), [io](https://docs.python.org/3/library/io.html), [shutil](https://docs.python.org/3/library/shutil.html), [tempfile](https://docs.python.org/3/library/tempfile.html)
- [gzip](https://docs.python.org/3/library/gzip.html), [zipfile](https://docs.python.org/3/library/zipfile.html), [hashlib](https://docs.python.org/3/library/hashlib.html), [importlib.resources](https://docs.python.org/3/library/importlib.resources.html)
- OWASP: Path Traversal

---
**Next:** [09 Configuration and Environment](../09-configuration/README.md)
