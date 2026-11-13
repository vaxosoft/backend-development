# P3 - File Service

**Modules exercised:** [02 HTTP server](../../modules/02-http-server/README.md), [08 File and stream I/O](../../modules/08-file-and-stream-io/README.md)

Build a small upload/download service on Fastify: multipart uploads **streamed to disk** (never fully buffered) while hashing, downloads streamed back with caching headers, and filename handling that cannot be used for path traversal.

## Requirements

| Request                                 | Result                                                                                                                                                                                                                      |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /files` (multipart, field `file`) | `201`, `Location: /files/{id}`, body `{"id","name","size","sha256","content_type"}`                                                                                                                                         |
| `GET /files/{id}`                       | `200`, the bytes streamed with `Content-Length`, `Content-Type` (as uploaded, default `application/octet-stream`), `Content-Disposition: attachment; filename="..."`, `ETag: "<sha256>"`, `X-Content-Type-Options: nosniff` |
| `GET /files/{id}` + `If-None-Match`     | matching ETag (also `W/"..."`, lists, `*`) gives `304` with the ETag and no body                                                                                                                                            |
| `DELETE /files/{id}`                    | `204`; removes everything stored for the file                                                                                                                                                                               |
| unknown or malformed id                 | `404 not_found`                                                                                                                                                                                                             |

Rules:

- Config: `storageDir` (created if missing) and `maxFileSize` (bytes, default 5 MiB = `DEFAULT_MAX_FILE_SIZE`). A larger upload is `413 payload_too_large` and must leave **nothing** on disk. A file of exactly `maxFileSize` is accepted.
- The upload is piped through a hashing/counting transform into a file: memory use does not depend on the file size.
- Files are stored as `<id>.bin` (+ `<id>.json` metadata) where `id` is 32 random hex characters. The client's name is only metadata, never a path. Ids that are not 32 lowercase hex characters are `404` before touching the disk.
- `sanitizeFilename`: keeps only the last path segment (`/` and `\` are separators), drops control characters and `"`, collapses `..` into `.`, trims, caps at 255 characters, and falls back to `file` for an empty or dots-only name. `../../etc/passwd` becomes `passwd`.
- Non-ASCII names: `filename="<ASCII fallback>"; filename*=UTF-8''<percent-encoded>` (header values must stay ASCII-safe).
- Errors use the shared envelope `{"error":{"code","message"}}`: `400 bad_request` (no `file` part), `415 unsupported_media_type` (body is not multipart), `413 payload_too_large`, `404 not_found`, `500 internal`.

## Acceptance criteria (test names in [tests/file-service.test.ts](./tests/file-service.test.ts))

| Area         | Tests                                                                                                                                                                                                                                   |
| ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Upload       | `stores the file and answers 201 with id, name, size, sha256 and content_type`, `each upload gets its own generated id ...`, `files live under generated ids in the storage directory ...`                                              |
| Traversal    | `a traversal name like ../../etc/passwd is sanitized and cannot escape the storage dir`, `sanitizeFilename removes separators, "..", control characters and quotes`, `unknown and malformed ids are 404 not_found, never a file lookup` |
| Limits       | `the default limit is 5 MiB ...`, `a file of exactly the configured maximum is accepted`, `an oversized upload is a 413 payload_too_large and leaves nothing on disk`                                                                   |
| Bad requests | `a request without a "file" part is a 400 bad_request`, `a body that is not multipart is a 415 unsupported_media_type`                                                                                                                  |
| Download     | `streams the bytes back with Content-Length, Content-Type, Content-Disposition and ETag`, `non-ASCII names get an ASCII fallback plus filename*`, `If-None-Match with the current ETag answers 304 with no body`                        |
| Delete       | `answers 204, removes everything from disk and the file is gone`                                                                                                                                                                        |
| Real socket  | `a 3 MiB upload and download keep their bytes and SHA-256`                                                                                                                                                                              |

## How to run

From `languages/typescript/`:

```bash
node projects/P3-file-service/starter/src/main.ts
```

Environment variables: `ADDR` (`host:port`, default `127.0.0.1:8080`), `STORAGE_DIR` (default `./data`), `MAX_FILE_SIZE` (bytes, default 5 MiB).

```bash
curl -F 'file=@notes.txt;type=text/plain' http://127.0.0.1:8080/files
curl -i http://127.0.0.1:8080/files/<id>
curl -i -H 'If-None-Match: "<sha256>"' http://127.0.0.1:8080/files/<id>
curl -X DELETE http://127.0.0.1:8080/files/<id>
```

With the reference solution the first command answers `{"id":"<32 hex>","name":"notes.txt","size":...,"sha256":"...","content_type":"text/plain"}`, the second shows the headers listed above, the third `HTTP/1.1 304 Not Modified`, and a file above `MAX_FILE_SIZE` gives `{"error":{"code":"payload_too_large","message":"file exceeds the maximum size"}}` with status `413`.

## How to run the tests

```bash
node --test "projects/P3-file-service/tests/**/*.test.ts"
```

Out of the box every test fails (`Error: TODO` surfaces as a `500`). Tests use `app.inject()` with multipart bodies built by `FormData` and one real listener on port 0; every test gets a fresh temporary directory.

## Suggested steps

1. [starter/src/filename.ts](./starter/src/filename.ts): `sanitizeFilename`.
2. [starter/src/store.ts](./starter/src/store.ts): `save` (stream to a `.part` file, hash while streaming, then rename), `find`, `open`, `remove`.
3. [starter/src/routes.ts](./starter/src/routes.ts): the three routes.
4. Run the tests after each step.

<details>
<summary>Hints</summary>

- `await req.file()` gives `{ file, filename, mimetype, fieldname }`; `file` is a `Readable`. Check `req.isMultipart()` first.
- `pipeline(source, transform, createWriteStream(path))` from `node:stream/promises` handles backpressure and rejects on any stream error; clean up the partial file in a `catch`.
- A `Transform` that calls `hash.update(chunk)` and counts bytes lets you hash and size the file in the same single pass.
- Past `limits.fileSize` the multipart parser **truncates** the stream (`file.truncated === true`) instead of failing it, so check the flag after the pipeline and remove what you stored.
- `createWriteStream(path, { flags: 'wx' })` refuses to overwrite an existing file.
- `reply.send(readStream)` streams; set `content-length` yourself so clients can show progress.
- A 304 response must not have a body; send the `ETag` header with it.
- `encodeURIComponent` does not escape `' ( ) *`; RFC 5987 requires them escaped in `filename*`.
- In your own tests, `app.server.closeAllConnections()` before `app.close()` avoids waiting for keep-alive sockets.

</details>

## Stretch goals (not tested)

- `HEAD /files/{id}`, and `Range` requests (`206 Partial Content`) via `createReadStream({ start, end })`.
- A `GET /files` listing with pagination, built from the `.json` metadata.
- Reject uploads early when `Content-Length` already exceeds the limit.
- Sweep leftover `*.part` files at startup.
- Store the hash in the file name and de-duplicate identical uploads.

## Reference solution

The model answer lives in [solution/](./solution/) (`filename.ts`, `store.ts`, `routes.ts`): copy its files over `starter/` (same relative paths) and the tests pass. Try the project yourself first.
