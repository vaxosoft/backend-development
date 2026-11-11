# P3. File Service

> Modules exercised: [02 HTTP server](../../modules/02-http-server/README.md), [08 File and stream I/O](../../modules/08-file-and-stream-io/README.md)

## Goal

Upload, download and delete files over HTTP **without ever buffering a whole file in memory**: the multipart body is read as a stream, hashed while it is written to disk, and refused as soon as it exceeds the size limit. Client-supplied file names are untrusted input and must never influence a path.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /files` | `multipart/form-data` with a part named `file`. Streams it to disk and answers `201` + `Location: /files/{id}` + `{"id","name","size","sha256","content_type"}` (`sha256` is lowercase hex of the content; `content_type` is the part's media type, default `application/octet-stream`). Other form fields are skipped. No `file` part or a non-multipart body: `400`. More than the maximum: `413` |
| `GET /files/{id}` | streams the content with `Content-Length`, `Content-Type`, `Content-Disposition: attachment; filename="<name>"` (plus `filename*` for UTF-8), `ETag: "<sha256>"`. A matching `If-None-Match` answers `304` without a body |
| `DELETE /files/{id}` | `204`; removes content and metadata |
| any unknown or malformed `{id}` | `404` (`application/problem+json`) |

Rules:

- **Limit**: `FileService:MaxUploadBytes`, default 5 MiB (5 242 880). Exactly the limit is accepted, one byte more is `413`. When the limit is exceeded nothing may be left on disk, and the server must stop reading the body.
- **Storage**: `FileService:StorageDir` (default `data`), created at startup. Files are stored as `{id}.bin` plus `{id}.json` metadata; `id` is a generated 32-character lowercase hex GUID. The client file name is stored only as display metadata.
- **Sanitising** (`FileNames.Sanitize`): keep the last segment after `/` or `\`, drop control characters, trim, truncate to 255 chars, and use `file` when the result is empty, `.` or `..`. `../../etc/passwd` becomes `passwd`.
- A defence in depth check: the resolved path of every file must be directly inside the storage directory.

> Errors use `application/problem+json` (RFC 9457), the idiomatic ASP.NET Core form of the track's error envelope.

## Acceptance criteria -> tests

| Criterion | Tests (`tests/FileServiceTests.cs`) |
|-----------|-------------------------------------|
| upload response | `Upload_Returns201WithMetadataAndLocation`, `Upload_ReportsTheSha256OfTheContent`, `Upload_AcceptsAnEmptyFile`, `Upload_WithoutPartContentType_DefaultsToOctetStream`, `Upload_UsesAFreshGeneratedIdEachTime` |
| multipart parsing | `Upload_SkipsOtherFormFieldsBeforeTheFile`, `Upload_WithoutAFilePart_Returns400`, `Upload_ThatIsNotMultipart_Returns400` |
| size limit | `Upload_EnforcesTheConfiguredLimit`, `Upload_TooLarge_Returns413ProblemJson`, `Upload_TooLarge_LeavesNothingOnDisk`, `Upload_DefaultLimitIs5MiB` |
| streaming | `Upload_IsStreamed_TheServerStopsReadingOnceTheLimitIsExceeded` |
| name sanitising | `Upload_SanitizesTheClientFileName`, `Upload_NeverWritesOutsideTheStorageDirectory` |
| download | `Download_ReturnsTheBytesWithTheExpectedHeaders`, `Download_UsesTheSanitizedNameInContentDisposition`, `Download_KeepsNonAsciiNamesIntact` |
| conditional GET | `Download_WithMatchingIfNoneMatch_Returns304WithoutBody`, `Download_WithDifferentIfNoneMatch_Returns200` |
| unknown / malformed ids | `Download_OfUnknownOrMalformedId_Returns404`, `Delete_OfUnknownOrMalformedId_Returns404` |
| delete | `Delete_Returns204RemovesTheFileAndItIsGone` |

## Run

From `languages/csharp/`:

```bash
export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
ADDR=127.0.0.1:8080 FileService__StorageDir=./data FileService__MaxUploadBytes=5242880 \
  dotnet run --project projects/P3-file-service/starter
```

`ADDR` defaults to `127.0.0.1:8080`. Try the reference solution:

```bash
printf 'hello file service\n' > hello.txt
curl -si -F 'file=@hello.txt;type=text/plain;filename=../../etc/hello.txt' localhost:8080/files
# HTTP/1.1 201 Created
# Location: /files/6741c49c5f664f029f203f0965a3323b          (id differs on every run)
# {"id":"6741c49c...","name":"hello.txt","size":19,"sha256":"8578d992884365a93343b565626b3ab21631194c322814c5b9539a8356b5d57e","content_type":"text/plain"}

curl -si localhost:8080/files/<id>
# HTTP/1.1 200 OK
# Content-Length: 19
# Content-Type: text/plain
# ETag: "8578d992884365a93343b565626b3ab21631194c322814c5b9539a8356b5d57e"
# Content-Disposition: attachment; filename="hello.txt"; filename*=UTF-8''hello.txt

curl -si -H 'If-None-Match: "8578d992884365a93343b565626b3ab21631194c322814c5b9539a8356b5d57e"' localhost:8080/files/<id>
# HTTP/1.1 304 Not Modified

head -c 6000000 /dev/zero > big.bin && curl -si -F 'file=@big.bin' localhost:8080/files
# HTTP/1.1 413 Payload Too Large
```

## Test

```bash
dotnet test projects/P3-file-service/tests
```

Tests use `WebApplicationFactory<Program>` with a private temporary storage directory (deleted afterwards). Only `Upload_IsStreamed_...` starts a real Kestrel on a loopback port, because the in-memory test server would consume the whole body. The starter compiles; `FileNames.cs`, `FileStore.cs` and `FileEndpoints.cs` contain `TODO` stubs that throw. Models, options, wiring (`Program.cs`) and `IsValidId` are given.

## Steps and hints

1. `FileNames.Sanitize` (pure function; try it first).
2. `FileStore.PathFor`, then `SaveAsync`, `Open`, `Delete`.
3. `FileEndpoints`: delete and download first, then the multipart upload.

<details>
<summary>Hints</summary>

- `Path.GetFileName` only understands the current OS's separators; `Split('/', '\\')[^1]` handles both. `char.IsControl` finds control characters.
- `Path.Combine("/data", "/etc/passwd")` returns `/etc/passwd` (a rooted second argument wins). That is why ids are validated *and* the result is checked with `Path.GetFullPath` against the root.
- Write to `{id}.part` first, `File.Move` it when complete, write the metadata last, and delete everything in a `catch` that rethrows.
- Hash while copying: `IncrementalHash.CreateHash(HashAlgorithmName.SHA256)`, `AppendData` per chunk, `GetHashAndReset` at the end, `Convert.ToHexStringLower`. Count bytes yourself and throw `FileTooLargeException` as soon as the count passes the limit.
- Do not use `IFormFile` or `Request.ReadFormAsync()`: they buffer the upload. Use `MultipartReader` (`Microsoft.AspNetCore.WebUtilities`) over `request.Body`, with the boundary from `MediaTypeHeaderValue.Parse(request.ContentType).Boundary`; `ContentDispositionHeaderValue` tells you the field name and file name of each section.
- Kestrel allows 30 MB by default and drains unread bodies. Set `IHttpMaxRequestBodySizeFeature.MaxRequestBodySize` (limit + a little for multipart framing) on the request so Kestrel itself stops reading, and treat its `BadHttpRequestException` with status 413 like your own limit error.
- `Results.Stream(stream, contentType, entityTag: new EntityTagHeaderValue("\"...\""))` sets `Content-Length` and `ETag`, answers `304` for a matching `If-None-Match`, and disposes the stream.
- Set `Content-Disposition` yourself (`Response.Headers.ContentDisposition`) to get `filename="..."`; encode non-ASCII names with `Uri.EscapeDataString` in `filename*`.

</details>

## Stretch goals (not tested)

- Support `Range` requests (`enableRangeProcessing: true`) and test partial downloads.
- Verify the content hash on download and reject corrupted files.
- Delete orphaned `*.part` files at startup.
- Add `GET /files` (list metadata with pagination) and a per-file `Content-Type` allow-list.
- Gzip downloads on the fly for compressible types (`System.IO.Compression`).

## Reference solution

`solution/` contains only the files that differ from `starter/` (`FileNames.cs`, `FileStore.cs`, `FileEndpoints.cs`). Try it yourself first; to check your work against it, `cp -r projects/P3-file-service/solution/. projects/P3-file-service/starter/` (this overwrites your changes).
