# P3: File service

**Modules exercised:** [02 HTTP server](../../modules/02-http-server/README.md), [08 File and stream I/O](../../modules/08-file-and-stream-io/README.md).

## Goal

Upload and download files **without ever holding a whole file in memory**: parse the multipart body incrementally, hash and count the bytes while they are written to disk, enforce a size limit early, never trust the client's file name, and serve files back with proper caching headers.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /files` | multipart field `file`; the bytes are streamed to disk. `201` `{"id","name","size","sha256","content_type"}` |
| `GET /files/{id}` | streams the file: `Content-Length`, `Content-Type` (as uploaded), `Content-Disposition: attachment; filename="<name>"`, `ETag: "<sha256>"`. A matching `If-None-Match` gives `304` without a body |
| `DELETE /files/{id}` | `204`; unknown id `404` |

Details:

- **Limit:** `Config.max_size` (default 5 MiB, env `MAX_UPLOAD_BYTES`). Exactly `max_size` bytes are accepted, one more is `413` `payload_too_large`. Reading stops as soon as the limit is crossed, and no partial file is left behind.
- **Ids:** generated (`uuid4().hex`, 32 lowercase hex characters). Ids that do not match that shape are `404` and never reach the file system.
- **Storage:** `Config.storage_dir` (env `STORAGE_DIR`, default `uploads`) holds `<id>.bin` plus a `<id>.json` metadata sidecar written last, so a half-written upload is never visible. Files survive restarts.
- **Names:** the client name is only metadata. Keep what follows the last `/` or `\`, drop control characters, replace `"`, trim, cap at 255 characters; a name that is empty or only dots becomes `file`. `../../etc/passwd` is stored as `passwd` under a generated id.
- **Content type:** from the part header, default `application/octet-stream`.
- **`If-None-Match`:** an exact ETag, a comma separated list, a weak `W/"..."` validator or `*` all match.
- **Errors:** the shared envelope. A body that is not `multipart/form-data`, has no file part called `file`, or is malformed is `400 bad_request`.

## Run

```bash
python -m pytest projects/P3-file-service/tests            # from languages/python
cd projects/P3-file-service/starter
ADDR=127.0.0.1:8080 STORAGE_DIR=uploads python -m p3_file_service
```

```bash
curl -s -F 'file=@notes.txt;type=text/plain' localhost:8080/files
curl -i localhost:8080/files/<id>
curl -i -H 'If-None-Match: "<sha256>"' localhost:8080/files/<id>
curl -i -X DELETE localhost:8080/files/<id>
```

## Acceptance criteria (tests in `tests/test_p3_files.py`)

| Criterion | Tests |
|-----------|-------|
| upload response, hash, content type | `test_upload_returns_201_with_id_name_size_hash_and_content_type`, `test_upload_without_a_part_content_type_defaults_to_octet_stream`, `test_upload_accepts_an_empty_file`, `test_upload_of_a_few_megabytes_keeps_every_byte` |
| generated ids, storage layout | `test_same_name_twice_gets_two_ids`, `test_files_are_stored_under_generated_ids` |
| sanitized names, no escape | `test_client_filename_is_sanitized_and_never_escapes_the_storage_dir` |
| size limit | `test_upload_exactly_at_the_limit_is_accepted`, `test_upload_over_the_limit_is_413_and_leaves_nothing_behind` |
| streaming (early stop) | `test_upload_stops_reading_the_body_once_the_limit_is_exceeded` |
| bad uploads | `test_upload_without_a_usable_file_part_is_400` |
| download headers | `test_download_streams_the_file_with_its_headers`, `test_download_of_a_non_ascii_or_spaced_name_keeps_a_quoted_filename` |
| conditional GET | `test_matching_if_none_match_returns_304_without_a_body`, `test_non_matching_if_none_match_returns_the_file` |
| unknown / malformed ids | `test_unknown_or_malformed_id_is_404` |
| persistence | `test_files_survive_a_restart` |
| delete | `test_delete_returns_204_and_removes_the_file` |

## What to implement

The starter contains the error handlers, `Config`, the `FileStore` skeleton and a complete incremental multipart reader (`multipart_stream.py`, read it: it is the streaming idea of this project). Every `# TODO` is yours:

| File | Work |
|------|------|
| `starter/p3_file_service/storage.py` | `sanitize_filename`, `FileStore.save`, `get`, `delete` |
| `starter/p3_file_service/app.py` | `content_disposition`, `etag_matches`, the three routes |

<details>
<summary>Hints</summary>

- `open_file_part(request, "file")` returns a `FilePart` whose `chunks()` is an async iterator; loop over it with `async for` and never collect the chunks in a list or `b"".join` them.
- Write to `<id>.part` first, then `Path.replace` it to `<id>.bin`. Catch `BaseException`, clean up, and re-raise: this also covers a client that disconnects (task cancellation).
- `hashlib.sha256().update(chunk)` is incremental; keep a running `size` and raise `FileTooLarge` the moment it exceeds the limit.
- `re.split(r"[\\/]", raw)[-1]` takes the last path component of both Unix and Windows style names. `unicodedata.category(ch) == "Cc"` finds control characters.
- `Response(status_code=304, headers={"ETag": ...})` has no body; `FileResponse(path, media_type=..., headers=...)` streams in 64 KiB chunks and sets `Content-Length` itself (explicit headers win).
- The writes inside `save` are small synchronous disk writes on the event loop: fine for this exercise, but a production service would hand them to a thread (`anyio.to_thread.run_sync`) or use a streaming-capable proxy.

</details>

## Stretch goals (not tested)

- Reject a request early when `Content-Length` already exceeds the limit (plus multipart overhead).
- `HEAD /files/{id}` and `Range` requests (`FileResponse` already supports ranges).
- Compress stored files with `gzip` and serve them decompressed.
- Verify the stored size and hash again on startup and report corrupt files.

## Reference solution

[`solution/`](./solution/) contains the two files that differ from the starter (`storage.py`, `app.py`). Try the project first, then compare.
