# 08. File and Stream I/O in TypeScript

> Estimated time: 1.5-2 h | Difficulty: Intermediate | Prerequisites: 01 (async/await), 06 (event loop, `AbortSignal`)

## Learning Outcomes

After this module you can:

- [ ] Read, write, append, list and delete files with `node:fs/promises` and handle `ENOENT`/`EEXIST` without race-prone `exists` checks
- [ ] Move large data with streams and `pipeline()` so memory stays flat and errors clean up every stage
- [ ] Hash, compress and encode data (`sha256`, gzip, base64, hex) while streaming
- [ ] Build paths safely, and keep untrusted file names from escaping a directory
- [ ] Use temp directories and ship static assets with your code

## 1. Concept Overview

A file is a sequence of bytes on disk; a **stream** is the same idea for any source of bytes (file, socket, HTTP body, compressor) delivered in chunks. Reading a 2 GB upload into memory with one call is the classic way to crash a service, so backend code reads, transforms and writes **chunk by chunk** and lets the destination slow the source down (**backpressure**).

```
source (file / request) -> transform (gunzip, parse lines, hash) -> sink (file / response)
        ^------------- backpressure: slow sink pauses the source -------------|
```

File names that come from users are untrusted input: `../../etc/passwd` is a valid name that must never become a path.

## 2. How TypeScript Approaches It

| Piece                                                                       | Use                                                                                              |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `node:fs/promises`                                                          | whole-file operations: `readFile`, `writeFile`, `appendFile`, `stat`, `readdir`, `mkdtemp`, `rm` |
| `node:fs` `createReadStream` / `createWriteStream`                          | streaming access to large files                                                                  |
| `node:stream` `Readable`, `Transform` and `node:stream/promises` `pipeline` | compose stages with backpressure and error propagation                                           |
| async generators (`async function*`)                                        | the modern way to write a transform stage: `for await` in, `yield` out                           |
| `node:readline`                                                             | line-by-line reading of a text stream                                                            |
| `node:zlib`, `node:crypto`, `Buffer`                                        | gzip, hashes, base64/hex                                                                         |
| `node:path`, `node:os`                                                      | paths and the temp directory                                                                     |

Idiomatic style: `await`able APIs from `fs/promises`, **`pipeline()` instead of `.pipe()`**, async iteration over streams, `try/catch` on the error `code` rather than checking existence first. Node can also run async generators as pipeline stages, so most transforms need no `Transform` class. Everything is built in; no libraries are added ([PROFILE](../../PROFILE.md)).

## 3. Guided Examples

Run all commands from `languages/typescript/`. Examples that write files use a fresh temp directory and delete it when done.

### 3.1 Reading and writing files

- **Goal:** the everyday `fs/promises` calls and their error codes.
- **Run:** `node modules/08-file-and-stream-io/examples/01_read_write_files.ts`
- **Code:** [`examples/01_read_write_files.ts`](examples/01_read_write_files.ts)

```ts
const dir = await mkdtemp(join(tmpdir(), 'm08-'));
await writeFile(file, 'first line\n', 'utf8');
await appendFile(file, 'second line\n');
await writeFile(file, 'overwrite?', { flag: 'wx' }); // EEXIST if it exists
```

- **Walkthrough:** without an encoding `readFile` returns a `Buffer`. The `wx` flag is an atomic "create only": no `exists()` check, so no race between check and write. Errors carry a `code` (`ENOENT`, `EEXIST`, `EACCES`) to branch on. `try/finally` with `rm(dir, { recursive: true, force: true })` guarantees cleanup.
- **Expected output:**

  ```
  "first line\nsecond line\n"
  23 bytes 6669727374
  true false 23
  wx -> EEXIST
  missing file -> ENOENT
  [ 'b.txt', 'notes.txt' ]
  ```

### 3.2 Streams and `pipeline`

- **Goal:** produce, transform and write 200,000 lines with flat memory and proper error handling.
- **Run:** `node modules/08-file-and-stream-io/examples/02_streams_pipeline.ts`
- **Code:** [`examples/02_streams_pipeline.ts`](examples/02_streams_pipeline.ts)

```ts
await pipeline(Readable.from(numberedLines(200_000)), upperCase, createWriteStream(file));
const lines = createInterface({ input: createReadStream(file), crlfDelay: Infinity });
for await (const line of lines) {
  /* one line at a time */
}
```

- **Walkthrough:** `Readable.from(asyncGenerator)` is a lazy source. `pipeline` connects the stages, pauses the source when the sink is full and, on any error, destroys **all** streams and rejects. The last block copies a missing file to show that rejection (`ENOENT`) and that no stream is leaked.
- **Expected output:**

  ```
  size 2288895
  200000 LINE 200000
  pipeline failed: ENOENT
  ```

### 3.3 Hashing, compression and encodings

- **Goal:** `sha256`, `gzip` and `base64`/`hex` on bytes and streams.
- **Run:** `node modules/08-file-and-stream-io/examples/03_hash_compress_encode.ts`
- **Code:** [`examples/03_hash_compress_encode.ts`](examples/03_hash_compress_encode.ts)

```ts
const hash = createHash('sha256');
for await (const chunk of source) hash.update(chunk);
hash.digest('hex');
Buffer.from(text, 'utf8').toString('base64url');
await pipeline(Readable.from([text]), createGzip(), createGunzip(), collect);
```

- **Walkthrough:** hashes are incremental, so a multi-gigabyte file is hashed in constant memory (feeding `'hel'` then `'lo'` equals hashing `'hello'`). `Buffer` is the encoding toolbox: `hex`, `base64`, `base64url`. The sync gzip helpers are fine for small payloads; use streaming `createGzip()` for large ones.
- **Expected output:**

  ```
  sha256("hello") = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  streamed        = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  hex        68c3a96c6c6f3f3e
  base64     aMOpbGxvPz4=
  base64url  aMOpbGxvPz4
  decoded    héllo?>
  gzip       5000 -> 50 true
  roundtrip  true
  ```

### 3.4 Paths and assets

- **Goal:** path arithmetic, a traversal-safe `resolveInside`, and files that ship with the code.
- **Run:** `node modules/08-file-and-stream-io/examples/04_paths_and_assets.ts`
- **Code:** [`examples/04_paths_and_assets.ts`](examples/04_paths_and_assets.ts), assets in [`examples/assets/`](examples/assets/)

```ts
const target = resolve(base, untrusted);
const rel = relative(base, target);
if (rel === '' || rel === '..' || rel.startsWith(`..${sep}`) || isAbsolute(rel))
  throw new Error('...');
```

- **Walkthrough:** `join('/srv/uploads', '../../etc/passwd')` silently returns a path outside the root, and `resolve` lets an absolute segment replace everything before it, so compare the _resolved_ path to the root with `relative`. A string `startsWith(root)` check is wrong (`/srv/up-evil` starts with `/srv/up`). Assets are located with `import.meta.dirname`, never with the current working directory; tiny static data can be embedded with `import data from './x.json' with { type: 'json' }`.
- **Expected output:**

  ```
  /srv/uploads/b/file.txt
  /etc/passwd
  report.final.pdf .pdf
  2030/report.pdf
  ok      report.pdf -> /srv/uploads/report.pdf
  ok      sub/dir/report.pdf -> /srv/uploads/sub/dir/report.pdf
  refused path escapes /srv/uploads: ../../etc/passwd
  refused path escapes /srv/uploads: /etc/passwd
  refused path escapes /srv/uploads: a/../../b
  Task Tracker export tool
  [ 'docs', 'p1' ] 1000
  ```

### 3.5 Task Tracker export: NDJSON + gzip

- **Goal:** stream 10,000 tasks to a compressed file and read them back, never holding them all in memory.
- **Run:** `node modules/08-file-and-stream-io/examples/05_task_export_ndjson_gzip.ts`
- **Code:** [`examples/05_task_export_ndjson_gzip.ts`](examples/05_task_export_ndjson_gzip.ts)

```ts
await pipeline(Readable.from(toNdjson(allTasks(10_000))), createGzip(), createWriteStream(file));
await pipeline(createReadStream(file), createGunzip(), splitLines, async (lines) => {
  for await (const line of lines) {
    /* JSON.parse(line) */
  }
});
```

- **Walkthrough:** `allTasks` stands in for a database cursor, `toNdjson` and `splitLines` are async-generator stages. `splitLines` uses `StringDecoder` so a multi-byte character cut by a chunk boundary is not corrupted. The reader is a `pipeline` too, so a corrupt gzip or a bad line rejects instead of hanging.
- **Expected output:**

  ```
  compressed file is non-empty: true
  first: Task 1
  per status: { in_progress: 3334, done: 3333, todo: 3333 }
  sha256 length: 64
  ```

## 4. Common Pitfalls

| Mistake                                                     | Why it happens                                                                      | Fix                                                                                                                        |
| ----------------------------------------------------------- | ----------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `readFile` on an upload or log of unknown size              | it is the shortest call and works on small test files                               | `createReadStream` + `pipeline`; reserve `readFile` for small, bounded files                                               |
| `a.pipe(b)`                                                 | the famous snippet from old tutorials                                               | `pipeline(a, b)`: `.pipe()` does not forward errors or destroy the other stream, which leaks file descriptors and can hang |
| Unhandled `'error'` event on a stream                       | streams are `EventEmitter`s: an unlistened `'error'` crashes the process            | use `pipeline`/`for await` so errors become rejections                                                                     |
| `readline` over a stream that can fail mid-iteration        | the interface re-emits input errors as an `'error'` event that nobody listens to    | for pipelines that may fail, split lines with an async generator stage                                                     |
| `if (await exists(p)) { await readFile(p) }`                | check-then-use is a race (`fs.exists` is deprecated for that reason)                | just call and catch `err.code === 'ENOENT'` (`wx` flag for create-only)                                                    |
| `join(root, userInput)` or `startsWith(root)`               | `join` happily returns paths outside `root`; prefix checks accept `/srv/up-evil`    | `resolve` then `relative`; reject `..`, absolute results and the root itself                                               |
| `chunk.toString()` on every chunk of UTF-8                  | a 2-4 byte character can be split across chunks                                     | `StringDecoder`, or `setEncoding('utf8')` on the stream                                                                    |
| Relative paths like `./assets/x.txt`                        | they resolve against `process.cwd()`, which depends on how the process was launched | `join(import.meta.dirname, 'assets', 'x.txt')`                                                                             |
| `writeFile` for crash-safe updates                          | a crash mid-write leaves a truncated file                                           | write to a temp file in the same directory, then `rename`                                                                  |
| Forgetting to clean up temp files and failed partial output | errors skip the happy-path `rm`                                                     | `try/finally` (or `rm` in the `catch` of the pipeline)                                                                     |

## 5. Coming From Other Languages

- **JavaScript:** the same `fs` APIs, but backend streams are Node streams, not the browser's WHATWG streams (`Readable.toWeb()` converts between them).
- **Java:** `Readable`/`Writable` ~ `InputStream`/`OutputStream`; `pipeline` ~ try-with-resources over a chain. Async iteration replaces the read loop with a buffer.
- **C#:** `Stream.CopyToAsync` does the pipe; `pipeline` additionally handles cancellation and destroys every stage on error.
- **Python:** `with open(...)` becomes `try/finally` or a `pipeline`; iterating a file line by line is `for await (const line of readline)`.
- **Go:** `io.Copy` and `io.Reader` pipelines correspond to `pipeline`; Node decides chunk sizes (64 KiB by default for files) and signals backpressure for you.

## 6. Exercises

Starters live in [`exercises/`](exercises/), reference solutions in [`solutions/`](solutions/). Run from `languages/typescript/`. To check a solution, copy its file over the starter (and restore the starter afterwards).

| #   | Title                                                            | Difficulty | Skill practiced                                           | Run tests                                                                            |
| --- | ---------------------------------------------------------------- | ---------- | --------------------------------------------------------- | ------------------------------------------------------------------------------------ |
| 1   | [Safe paths](exercises/ex1_safe_paths/safe-paths.ts)             | Easy       | `node:path`, traversal, sanitising names                  | `node --test "modules/08-file-and-stream-io/exercises/ex1_safe_paths/*.test.ts"`     |
| 2   | [Copy with hash](exercises/ex2_copy_with_hash/copy-with-hash.ts) | Medium     | `pipeline`, `Transform`, cleanup on failure               | `node --test "modules/08-file-and-stream-io/exercises/ex2_copy_with_hash/*.test.ts"` |
| 3   | [NDJSON gzip](exercises/ex3_ndjson_gzip/ndjson-gz.ts)            | Stretch    | async generators, gzip, line splitting, error propagation | `node --test "modules/08-file-and-stream-io/exercises/ex3_ndjson_gzip/*.test.ts"`    |

**Exercise 1 - Safe paths**

- **Task:** implement `resolveInside(root, untrusted)` and `sanitizeFilename(name)` (this is the filename handling the P3 file service needs).
- **Acceptance:** all tests in `safe-paths.test.ts` pass, including the sibling-prefix case.
- <details><summary>Hint</summary>Resolve first, then ask `relative(root, target)` where the target is; it must not start with `..` and must not be empty.</details>

**Exercise 2 - Copy with hash**

- **Task:** implement `copyWithSha256(src, dest)` as a stream copy that also returns the byte count and SHA-256, and leaves no `dest` behind on failure.
- **Acceptance:** all tests in `copy-with-hash.test.ts` pass (3 MiB copy, empty file, overwrite, missing source).
- <details><summary>Hint</summary>A `Transform` placed between the read and write streams can update the hash and a counter without changing the chunks. `createWriteStream` creates the destination before it knows the source is missing.</details>

**Exercise 3 - NDJSON gzip**

- **Task:** implement `writeNdjsonGz` (lazy iterable in, gzip file out) and `forEachNdjsonGz` (stream back with 1-based line numbers, blank lines skipped, clear error for bad JSON, stop when the handler throws).
- **Acceptance:** all tests in `ndjson-gz.test.ts` pass.
- <details><summary>Hint</summary>Re-read example 3.5: `Readable.from(generator)` for writing, and a `pipeline` ending in an async function for reading. Put the line splitting into an async generator stage.</details>

## 7. Recap

- Use `fs/promises` for small files and **streams with `pipeline()`** for anything of unbounded size.
- Backpressure is automatic when you use `pipeline`, `for await` or `Readable.from(asyncGenerator)`; manual `.on('data')` code gives it up.
- Handle `err.code` instead of pre-checking; clean up temp and partial files in `finally`.
- Resolve then compare (`relative`) to confine paths; sanitise names and never use a client name as a path.
- Hashing, gzip and encodings all work incrementally, so they compose into the same pipeline.

## 8. Self-check Questions

1. A pipeline stage fails halfway. What state are the source and destination streams in with `pipeline`, and what with `.pipe()`?
2. Why is `resolve(root, input).startsWith(root)` not a safe containment check?
3. What does the `wx` flag give you that `if (!exists) write` does not?
4. Where does memory stay constant in example 3.5, and which line would break that guarantee if replaced by `readFile`?
5. When is it fine to use `gzipSync` in a server, and when is it not?

## 9. Further Reading

- [Node.js docs: `stream`](https://nodejs.org/api/stream.html) (backpressure guide under "Learn": _Backpressuring in Streams_)
- [Node.js docs: `fs/promises`](https://nodejs.org/api/fs.html#promises-api), [`zlib`](https://nodejs.org/api/zlib.html), [`crypto` hashes](https://nodejs.org/api/crypto.html#class-hash)
- [OWASP: Path Traversal](https://owasp.org/www-community/attacks/Path_Traversal)

---

**Next:** [09 Configuration](../09-configuration/README.md)
