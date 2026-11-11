# 08. File and Stream I/O in C#

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 06

## Learning Outcomes
After this module you can:
- [ ] Read and write files with the whole-file helpers and with streams, and choose between them.
- [ ] Process data of any size in constant memory (copy, hash, compress, read line by line).
- [ ] Build paths safely and keep untrusted names inside a root directory.
- [ ] Compress with gzip/zip, encode with Base64/hex, and hash a stream with SHA-256.
- [ ] Embed an asset in the assembly and replace a file atomically.

## 1. Concept Overview
Everything that moves bytes (files, sockets, HTTP bodies, compressors) is a **stream**: a sequence you read or write in pieces. Code written against the stream abstraction does not care where the bytes come from, and it can handle a 10 GB upload with a few kilobytes of memory. Whole-file helpers (`File.ReadAllText`) are fine for small data; streams are for everything that can be large or must be processed while it arrives.

Streams compose like decorators: `FileStream` -> `GZipStream` -> `StreamWriter` turns text into compressed bytes on disk. The two recurring hazards are **forgetting to flush/dispose** (data is lost or invisible) and **trusting a path** that came from a user.

## 2. How C# Approaches It
| Type | Role |
|------|------|
| `File`, `Directory`, `Path`, `FileInfo` | Static helpers for whole files, directory operations and path string manipulation |
| `FileStream` (+ `FileStreamOptions`, `FileOptions.Asynchronous`) | Byte stream over a file; control mode, sharing, buffer size and async I/O |
| `StreamReader` / `StreamWriter` | Text over a stream (UTF-8 by default) |
| `Stream.CopyToAsync`, `ReadAsync(Memory<byte>)` | The async, buffer-based building blocks |
| `GZipStream`, `ZipArchive` (`System.IO.Compression`) | Compression decorators |
| `Convert`, `System.Buffers.Text.Base64Url`, `SHA256.HashDataAsync` | Encoding and hashing, also stream based |
| `EmbeddedResource` + `Assembly.GetManifestResourceStream` | Assets shipped inside the DLL |

Idiomatic: `await using` for every stream and the async methods (`WriteAllTextAsync`, `ReadAsync`) in server code; non-idiomatic: `File.ReadAllBytes` on user uploads, sync I/O on request threads, `Path.Combine` with user input, forgetting that disposing the *writer* flushes it. Only the BCL is used in this module.

## 3. Guided Examples
Run from `languages/csharp/`: `dotnet run --project modules/08-file-and-stream-io/examples -- <name>`. All examples work in a self-deleting temp directory. Output shows `/` separators (Linux/macOS).

### 3.1 Whole-file helpers
- **Goal:** the quick API, plus directories and encodings.
- **Run:** `dotnet run --project modules/08-file-and-stream-io/examples -- 01-read-write-files`
- **Code:** [examples/01_read_write_files.cs](examples/01_read_write_files.cs)
- **Walkthrough:** `WriteAllTextAsync`/`AppendAllTextAsync`/`ReadAllLinesAsync`, `Directory.CreateDirectory` (creates all levels, idempotent), `File.Copy/Move/Delete`, `EnumerateFiles`. "café ☕" is 6 characters but 9 UTF-8 bytes: sizes are bytes.
- **Expected output:**
  ```
  3 lines: buy milk | write lesson | ship it
  size on disk: 30 bytes
  'café ☕' = 6 chars = 9 UTF-8 bytes
  notes.txt exists after move: False
  found exports/2030/notes-copy.txt
  found exports/2030/notes-moved.txt
  reading a missing file throws FileNotFoundException
  ```

### 3.2 Streams, buffering and constant memory
- **Goal:** write and read 100,000 lines without holding them in memory; see why flushing matters.
- **Run:** `dotnet run --project modules/08-file-and-stream-io/examples -- 02-streams`
- **Code:** [examples/02_streams.cs](examples/02_streams.cs)
  ```csharp
  await using (var file = new FileStream(path, new FileStreamOptions { Mode = FileMode.Create, Access = FileAccess.Write, Options = FileOptions.Asynchronous }))
  await using (var writer = new StreamWriter(file, new UTF8Encoding(false)))
  { for (var i = 1; i <= 100_000; i++) await writer.WriteLineAsync($"line {i}"); }
  ```
- **Walkthrough:** disposing the writer flushes its buffer into the `FileStream`, which flushes to disk. `CountBytesAsync(Stream)` works unchanged on a file and on a `MemoryStream`. The last block shows that unflushed data is invisible to other readers.
- **Expected output:**
  ```
  wrote 1088895 bytes
  read 100000 lines, last = 'line 100000'
  CountBytes(file)   = 1088895
  CountBytes(memory) = 13
  before flush: 0 bytes on disk
  after flush:  7 bytes on disk
  ```

### 3.3 Paths and traversal safety
- **Goal:** know the `Path.Combine` trap and check that a resolved path stays inside the root.
- **Run:** `dotnet run --project modules/08-file-and-stream-io/examples -- 03-paths-and-safety`
- **Code:** [examples/03_paths_and_safety.cs](examples/03_paths_and_safety.cs)
  ```csharp
  var rootFull = Path.TrimEndingDirectorySeparator(Path.GetFullPath(root)) + Path.DirectorySeparatorChar;
  return Path.GetFullPath(candidate).StartsWith(rootFull, StringComparison.Ordinal); // trailing separator: "/data-evil" is not inside "/data"
  ```
- **Walkthrough:** `Path.Combine(root, "/etc/passwd")` *discards* `root`; `Path.Join` keeps it. `TryResolve` joins, normalizes with `GetFullPath` (collapsing `..`) and only then compares prefixes. `/etc/passwd` is "allowed" because it is treated as `<root>/etc/passwd`, inside the root. For uploads, never use the client name as a path: store under a generated id and keep the sanitized name as metadata.
- **Expected output:**
  ```
  -- Path.Combine vs Path.Join (second argument is rooted)
  Combine(root, "/etc/passwd") is inside root: False
  Join(root, "/etc/passwd")    is inside root: True
  -- traversal check
  report.txt             -> allowed
  sub/report.txt         -> allowed
  ../secret.txt          -> rejected
  a/../../secret.txt     -> rejected
  /etc/passwd            -> allowed
  ../uploads-evil/x      -> rejected
  -- storing under a generated id, the client name is only metadata
  client name '../../etc/passwd' -> display name 'passwd'
  stored in root: True, file count in root: 1
  ```

### 3.4 Compression, encoding and hashing
- **Goal:** gzip/zip round trips, Base64/hex, and SHA-256 of a stream.
- **Run:** `dotnet run --project modules/08-file-and-stream-io/examples -- 04-compress-encode-hash`
- **Code:** [examples/04_compress_encode_hash.cs](examples/04_compress_encode_hash.cs)
- **Walkthrough:** the `GZipStream` must be disposed before you read the compressed bytes (the trailer is written on dispose); `leaveOpen: true` keeps the underlying stream usable. `Base64Url` uses `-`/`_` and no padding (JWTs, URLs). `SHA256.HashDataAsync(stream)` never loads the whole input; compare digests with `CryptographicOperations.FixedTimeEquals`.
- **Expected output:**
  ```
  gzip smaller: True, round trip equal: True
  zip entry tasks.json: content of tasks.json
  zip entry notes/readme.txt: content of notes/readme.txt
  hex        fbfffe002a
  base64     +//+ACo=
  base64url  -__-ACo
  decoded    fbfffe002a
  sha256     2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
  fixed-time compare: True
  ```

### 3.5 Task archive: embedded seed, atomic gzip JSON Lines
- **Goal:** the realistic combination: read an embedded resource, export tasks as gzipped JSON Lines, replace the file atomically, stream it back.
- **Run:** `dotnet run --project modules/08-file-and-stream-io/examples -- 05-task-archive`
- **Code:** [examples/05_task_archive.cs](examples/05_task_archive.cs) (seed: [Assets/seed-tasks.json](examples/Assets/seed-tasks.json), embedded via `<EmbeddedResource ... LogicalName="seed-tasks.json" />`)
- **Walkthrough:** `TaskArchive.WriteAsync` writes `<path>.<guid>.tmp` in the *same directory*, disposes (flushes) every layer, then `File.Move(temp, path, overwrite: true)`; on any exception the temp file is deleted, so a failed export leaves the previous archive intact. `ReadAsync` is an `IAsyncEnumerable<TaskItem>` over `StreamReader.ReadLineAsync`.
- **Expected output:**
  ```
  embedded seed holds 3 tasks
    #1 Write docs [Todo]
    #2 Review "streams" lesson [InProgress]
    #3 Ship café menu ☕ [Done]
  sha256 of the JSON Lines content: 49c0bd2513b0373140b883d5a0cbf3048bdfc73b2eedc617cde621fe0d710f75
  second export failed: database went away
  archive still has 3 tasks; files in directory: tasks.jsonl.gz
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `Path.Combine(root, userInput)` | A rooted second argument (`/etc/passwd`, `C:\x`) silently replaces the first | `Path.Join`, then `GetFullPath` and a prefix check that includes the trailing separator |
| `fullPath.StartsWith(root)` without a separator | `/data-evil` starts with `/data` | Compare against `root + Path.DirectorySeparatorChar` (ordinal) |
| Treating `\` as a normal character | On Linux `..\..\x` is one file name, but the same string traverses on Windows | Normalize both separators before checking; or never use client names as paths |
| Data missing from the file | `StreamWriter`/`FileStream` buffer; disposing the inner stream first, or not at all, loses data | `await using` both (writer disposes in reverse order) or `FlushAsync` |
| Reading a `GZipStream`'s output before disposing it | The gzip trailer/final block is written on dispose | Dispose (use a block scope) before reading; `leaveOpen: true` for the target |
| `File.ReadAllBytes`/`ReadAllText` for uploads or logs | Whole file in memory; large-object-heap pressure | `CopyToAsync`, `ReadLineAsync`, `SHA256.HashDataAsync(stream)` |
| Sync I/O (`File.ReadAllText`, `stream.Read`) in async request code | Blocks a thread-pool thread | Async APIs and `FileOptions.Asynchronous` |
| Overwriting a file in place | A crash or exception mid-write leaves a truncated file | Write to a temp file in the same directory, then `File.Move(..., overwrite: true)` |
| `new FileStream(path, FileMode.Create)` for a file others read | Default sharing/locking differs by OS | State `FileShare` explicitly |
| Assuming `string.Length` is the byte size | UTF-16 chars vs UTF-8 bytes | `Encoding.UTF8.GetByteCount` or `FileInfo.Length` |

## 5. Coming From Other Languages
- **Go:** `io.Reader`/`io.Writer` ~ `Stream`; `io.Copy` ~ `CopyToAsync`; `defer f.Close()` ~ `using`/`await using`; `bufio.Scanner` ~ `StreamReader.ReadLineAsync`.
- **Java:** `InputStream`/`OutputStream` decorators map directly (`GZIPOutputStream` ~ `GZipStream`); try-with-resources ~ `using`; `Files.readAllLines` ~ `File.ReadAllLinesAsync`; `Path.resolve` has the same rooted-argument surprise.
- **TypeScript/JS:** Node `stream.pipeline` ~ chaining decorators and `CopyToAsync`; `fs.promises` ~ the `*Async` helpers; `for await (const line of rl)` ~ `await foreach` over `IAsyncEnumerable`.
- **Python:** `with open(...)` ~ `using`; `shutil.copyfileobj` ~ `CopyToAsync`; `hashlib.update` loop ~ `SHA256.HashDataAsync`.

## 6. Exercises
Run one exercise at a time from `languages/csharp/`. Solutions live in [solutions/](solutions/) (copy over the starter to check).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Safe paths | Easy | Traversal-proof path resolution and file-name sanitizing | `dotnet test modules/08-file-and-stream-io/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Upload store | Medium | Stream to disk with a size cap, incremental SHA-256, cleanup on failure | `dotnet test modules/08-file-and-stream-io/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Tail reader | Stretch | `Seek` backwards and read chunks from the end of a stream | `dotnet test modules/08-file-and-stream-io/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: [Ex1_SafePath.cs](exercises/Ex1_SafePath.cs)
- **Task:** implement `SafePath.Resolve` (reject anything that escapes the root, treat a leading `/` or `\` as relative) and `SafePath.SanitizeFileName`.
- **Acceptance:** `Ex1_SafePathTests` (inside paths, `..` escapes, sibling-prefix `data-evil`, NUL, display-name rules).
- **Hint:** <details><summary>Show</summary>Normalize `\` to `/`, trim leading `/`, `Path.GetFullPath(Path.Join(root, rel))`, then require the result to equal the root or start with `root + separator`.</details>

### Exercise 2: [Ex2_UploadStore.cs](exercises/Ex2_UploadStore.cs)
- **Task:** `UploadStore.SaveAsync` streams to a generated-id file while hashing, enforces `maxBytes` without reading an endless input, and leaves nothing behind on failure; `Open` only accepts generated ids.
- **Acceptance:** `Ex2_UploadStoreTests` (hash/size, chunk boundaries, exact limit, endless stream, cancellation, id validation).
- **Hint:** <details><summary>Show</summary>`IncrementalHash.CreateHash(SHA256)` + a read loop; throw as soon as the running total exceeds the cap; delete the file in a `catch { ...; throw; }`.</details>

### Exercise 3: [Ex3_TailReader.cs](exercises/Ex3_TailReader.cs)
- **Task:** `TailReader.LastLinesAsync(stream, count)` returns the last lines without reading the whole stream.
- **Acceptance:** `Ex3_TailReaderTests` (line endings, short files, a 200,000-line stream where less than 5% may be read, long and multi-byte lines).
- **Hint:** <details><summary>Show</summary>Seek to `Length - chunk`, read, count `\n` bytes going backwards until you have `count + 1` (a final newline does not count), then decode the collected tail as UTF-8; `\n` never occurs inside a multi-byte UTF-8 sequence.</details>

## 7. Recap
- Streams are the abstraction; helpers (`File.*`) are conveniences for small data.
- Dispose (flush) in the right order with `await using`; never assume buffered data is on disk.
- Treat every path from outside as hostile: `Path.Join` + `GetFullPath` + separator-aware prefix check, or store under generated ids.
- Hash, compress and copy as streams; replace files atomically via temp file + move.
- When to reach for what: `File.*` for config-sized files, `FileStream`/`CopyToAsync` for uploads and exports, `IAsyncEnumerable` over a reader for large line-based data.

## 8. Self-check Questions
1. What does `Path.Combine("/srv/data", "/etc/passwd")` return and why is it dangerous with user input?
2. Why does the traversal check append a directory separator to the root before `StartsWith`?
3. Your export writes through `StreamWriter` -> `GZipStream` -> `FileStream` and the archive is truncated. Which disposal or flush step is missing?
4. Why is writing a temp file in the *same directory* and then moving it atomic enough, whereas writing to `/tmp` first might not be?
5. How would you cap an upload at 5 MiB without ever buffering it fully, and what must happen to the partial file?

## 9. Further Reading
- [File and stream I/O](https://learn.microsoft.com/dotnet/standard/io/) and [Path class](https://learn.microsoft.com/dotnet/api/system.io.path)
- [Compression and extraction](https://learn.microsoft.com/dotnet/standard/io/compress-and-extract-files-in-dotnet)
- [OWASP: Path traversal](https://owasp.org/www-community/attacks/Path_Traversal)

---
**Next:** [09 Configuration](../09-configuration/README.md)
