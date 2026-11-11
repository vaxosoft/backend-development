# P3 - File Service

> **Modules exercised:** [02 HTTP Server](../../modules/02-http-server/README.md), [08 File and Stream I/O](../../modules/08-file-and-stream-io/README.md)
> **Time:** 2-3 hours · **Difficulty:** Intermediate

## Goal

Build an upload/download service that **streams** files to and from disk (memory use independent of file size), enforces a size limit, serves cache-friendly downloads with an `ETag`, and treats every client-supplied name as hostile.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /files` | `multipart/form-data` with the file in field `file`; `201`, `Location: /files/{id}`, body `{"id","name","size","sha256","content_type"}` |
| `GET /files/{id}` | streams the bytes with `Content-Length`, `Content-Type`, `Content-Disposition: attachment; filename="<name>"` and `ETag: "<sha256>"`. `If-None-Match` matching the ETag (also `*`, a comma list, or a `W/` form) -> `304` with the `ETag` and no body |
| `DELETE /files/{id}` | `204`; removes the data **and** the metadata |
| Errors | JSON envelope `{"error":{"code","message"}}`: `400 bad_request` (not multipart, or no `file` part), `404 not_found`, `413 payload_too_large`, `500 internal` |

Rules:

- **Streaming**: use `r.MultipartReader()` (not `ParseMultipartForm`) and copy the part straight to a temp file through `io.MultiWriter(file, sha256)`. Never `io.ReadAll` an upload. Other form fields before the file are skipped.
- **Size limit**: `Config.MaxSize` (default 5 MiB; env `MAX_UPLOAD_BYTES` in `main`). A file of exactly `MaxSize` bytes is accepted, one byte more is `413`. The request body is also capped with `http.MaxBytesReader` (`MaxSize` + 64 KiB for multipart framing) so a client cannot stream gigabytes. A rejected upload leaves **no** file behind, not even a temporary one.
- **Ids and storage**: files live in `Config.Dir` under random 32-character lowercase hex ids (`<id>.blob` + `<id>.json`, written via temp file + rename). Files survive a restart. An id that is not 32 lowercase hex characters is `404` and must never reach the filesystem (`/files/..%2Fsecret` decodes to `../secret` in `r.PathValue`).
- **Filenames**: `SanitizeFilename` keeps what follows the last `/` or `\`, drops control characters and `"`, trims spaces, turns `""`/`.`/`..` into `file`, and keeps at most 100 runes. The result is metadata only, never part of a path. `../../etc/passwd` is stored as `passwd`, inside the storage directory.
- `content_type` is the part's `Content-Type` if it parses, else `application/octet-stream`.
- Always check the error from `Close()` on the file you wrote.

## Layout

```
projects/P3-file-service/
├── starter/            package starter (+ cmd/server/main.go, complete)
│   ├── app.go          Config, New wiring and routes - complete
│   ├── sanitize.go     TODO SanitizeFilename
│   ├── storage.go      Meta, NewStorage, NewID given; TODO Save, Open, Delete
│   └── handlers.go     helpers given; TODO upload, download, remove
├── _solution/           sanitize.go, storage.go, handlers.go (overlay)
└── tests/              black-box tests over real HTTP, temp dirs (never overlaid)
```

## Acceptance criteria

| Criterion | Tests |
|-----------|-------|
| Upload: 201, metadata, `Location`, sha256 | `TestUploadReturns201WithMetadataAndLocation`, `TestEachUploadGetsItsOwnID` |
| Content type, other fields, empty file | `TestUploadWithoutContentTypeDefaultsToOctetStream`, `TestUploadSkipsOtherFieldsBeforeTheFilePart`, `TestUploadAcceptsAnEmptyFile` |
| Bad uploads -> 400 | `TestUploadWithoutFilePartIs400`, `TestUploadThatIsNotMultipartIs400` |
| Size limit -> 413, no leftovers, boundaries, default 5 MiB | `TestUploadOverTheConfiguredLimitIs413AndLeavesNoFiles`, `TestUploadExactlyAtTheLimitIsAcceptedOneByteMoreIsNot`, `TestUploadFarOverTheLimitIs413`, `TestDefaultLimitIs5MiB` |
| Download bytes and headers | `TestDownloadStreamsExactBytesWithHeaders`, `TestDownloadHeaderUsesTheSanitizedName` |
| Conditional GET | `TestDownloadWithMatchingIfNoneMatchIs304WithoutBody`, `TestDownloadWithDifferentIfNoneMatchReturnsTheFile` |
| Unknown / malformed / traversal ids | `TestDownloadUnknownIDIs404WithEnvelope`, `TestMalformedAndTraversalIDsAre404` |
| Delete | `TestDeleteReturns204RemovesTheFileAndItsDiskSpace`, `TestDeleteUnknownIDIs404` |
| Filename safety | `TestSanitizeFilename`, `TestHostileFilenameNeverEscapesTheStorageDirectory` |
| Persistence | `TestFilesSurviveARestartOnTheSameDirectory` |

## How to run

From `languages/go/`:

```bash
ADDR=127.0.0.1:8080 FILES_DIR=./data go run ./projects/P3-file-service/starter/cmd/server
go test ./projects/P3-file-service/tests/            # fails until you are done
go test -race -count=1 ./projects/P3-file-service/tests/
```

`ADDR` defaults to `127.0.0.1:8080`, `FILES_DIR` to `data`, `MAX_UPLOAD_BYTES` to 5 MiB. Try it (ids and hashes below come from a real run; the id differs on every upload):

```bash
curl -si -F 'file=@hello.txt;type=text/plain' localhost:8080/files
# HTTP/1.1 201 Created
# Location: /files/2a4275de08a7780d9ed5df15d1681ad4
# {"id":"2a4275de08a7780d9ed5df15d1681ad4","name":"hello.txt","size":19,"sha256":"8578d992...d57e","content_type":"text/plain"}

curl -si localhost:8080/files/2a4275de08a7780d9ed5df15d1681ad4
# HTTP/1.1 200 OK
# Content-Disposition: attachment; filename="hello.txt"
# Content-Length: 19
# Content-Type: text/plain
# Etag: "8578d992884365a93343b565626b3ab21631194c322814c5b9539a8356b5d57e"
#
# hello file service

curl -si -H 'If-None-Match: "8578d992884365a93343b565626b3ab21631194c322814c5b9539a8356b5d57e"' localhost:8080/files/2a4275de08a7780d9ed5df15d1681ad4
# HTTP/1.1 304 Not Modified

curl -s -F 'file=@hello.txt;filename=../../etc/passwd' localhost:8080/files
# {"id":"...","name":"passwd",...}

curl -s -X DELETE -w '%{http_code}\n' localhost:8080/files/2a4275de08a7780d9ed5df15d1681ad4   # 204
```

(`MAX_UPLOAD_BYTES=1024` and a 5000-byte upload gives `{"error":{"code":"payload_too_large",...}}` with status `413`.)

## Suggested steps

1. `SanitizeFilename` (pure function; `TestSanitizeFilename` first).
2. `Storage.Save`, `Open`, `Delete`, then the `upload`, `download` and `remove` handlers (write `upload` first if you want to see 201s early).
3. Upload limits and error mapping (`413`, `400`).
4. ETag / `If-None-Match`.
5. Run the suite with `-race`.

<details>
<summary>Hints</summary>

- `r.MultipartReader()` returns an error unless the request is `multipart/*`; `mr.NextPart()` returns `io.EOF` after the last part. `part` is an `io.Reader`: pass it to `Save` directly.
- `io.LimitReader(r, limit+1)` and comparing the copied byte count with `limit` distinguishes "exactly at the limit" from "over it" without reading the rest.
- Reading a part that crosses the `MaxBytesReader` limit fails with `*http.MaxBytesError`; `errors.As` finds it even when wrapped with `%w`.
- `os.CreateTemp(dir, ".upload-*")` creates the file with mode `0600`. Create it in the **same directory** as the final file: `os.Rename` is only atomic within one filesystem. Remove it on every error path (a `defer` with a named `err` result keeps this in one place).
- `hex.EncodeToString(hasher.Sum(nil))` is the digest; set `ETag` to that value in double quotes.
- Set `Content-Length` yourself from `Meta.Size`, then `io.Copy(w, f)`. After the first byte the status can no longer change: just log a copy error.
- Header values are not escaped for you: that is why `"` and control characters are dropped from names.
- Validate the id **before** building any path with `filepath.Join`; `Join` happily resolves `..`.

</details>

## Stretch goals (not tested)

- Support `Range` requests (`http.ServeContent` does it, including conditional requests, for any `io.ReadSeeker`).
- Delete files older than a configurable age with a background goroutine.
- Store the hash-addressed blob once for identical uploads (content-addressable storage).
- Gzip-compress text types on download (`compress/gzip`, `Accept-Encoding`).

## Reference solution

[`_solution/`](./_solution/) holds `sanitize.go`, `storage.go` and `handlers.go`. Copy over the starter to see every test pass:

```bash
cp -r projects/P3-file-service/_solution/. projects/P3-file-service/starter/
```
