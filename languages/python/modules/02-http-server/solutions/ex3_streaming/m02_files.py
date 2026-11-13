"""Exercise 3: stream uploads to disk and downloads from disk without buffering whole files."""

import hashlib
import re
import uuid
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path, PurePosixPath

from fastapi import FastAPI, Request, Response, UploadFile
from fastapi.responses import JSONResponse, StreamingResponse

CHUNK_SIZE = 64 * 1024
UNSAFE = re.compile(r"[^\w.\- ]")


def sanitize_filename(raw: str | None) -> str:
    """Reduce a client-supplied filename to a harmless display name.

    - keep only the last path component (both "/" and "\\" separate components)
    - replace every character that is not a word character, ".", "-" or space with "_"
    - strip leading dots (no hidden files, no "..")
    - fall back to "upload.bin" when nothing is left (or `raw` is None)
    """
    last = PurePosixPath((raw or "").replace("\\", "/")).name
    return UNSAFE.sub("_", last).lstrip(".").strip() or "upload.bin"


@dataclass(slots=True, frozen=True)
class StoredFile:
    name: str
    content_type: str
    size: int


def error(status: int, code: str, message: str) -> JSONResponse:
    return JSONResponse({"error": {"code": code, "message": message}}, status_code=status)


def create_app(storage_dir: Path, max_bytes: int = 5 * 1024 * 1024) -> FastAPI:
    """Routes (errors use {"error": {"code": ..., "message": ...}}):

    POST /files   multipart field "file". Read it in CHUNK_SIZE pieces, hashing and writing to a
                  temporary file inside `storage_dir`. More than `max_bytes` -> 413 with code
                  "payload_too_large" and NO file left behind. Otherwise move the temporary file
                  to storage_dir/<sha256> and answer 201 with header Location: /files/<sha256> and
                  {"id": <sha256>, "name": <sanitized name>, "size": <bytes>,
                   "sha256": <sha256>, "content_type": <part content type>}.
    GET /files/{id}
                  unknown id -> 404 (code "not_found"). Otherwise stream the file in chunks with
                  Content-Type (as uploaded), Content-Length, ETag: "<sha256>" and
                  Content-Disposition: attachment; filename="<sanitized name>".
                  If-None-Match equal to the ETag -> 304 with an empty body.
    """
    app = FastAPI()
    storage_dir.mkdir(parents=True, exist_ok=True)
    stored: dict[str, StoredFile] = {}

    # A plain `def` handler runs in a worker thread, so blocking file I/O is fine here.
    @app.post("/files", status_code=201, response_model=None)
    def upload(file: UploadFile) -> Response:
        digest = hashlib.sha256()
        size = 0
        partial = storage_dir / f"upload-{uuid.uuid4().hex}.part"
        try:
            with partial.open("wb") as out:
                while chunk := file.file.read(CHUNK_SIZE):
                    size += len(chunk)
                    if size > max_bytes:
                        return error(413, "payload_too_large", f"limit is {max_bytes} bytes")
                    digest.update(chunk)
                    out.write(chunk)
            file_id = digest.hexdigest()
            partial.replace(storage_dir / file_id)
        finally:
            partial.unlink(missing_ok=True)

        info = StoredFile(sanitize_filename(file.filename), file.content_type or "", size)
        stored[file_id] = info
        return JSONResponse(
            {
                "id": file_id,
                "name": info.name,
                "size": size,
                "sha256": file_id,
                "content_type": info.content_type,
            },
            status_code=201,
            headers={"Location": f"/files/{file_id}"},
        )

    @app.get("/files/{file_id}", response_model=None)
    def download(file_id: str, request: Request) -> Response:
        info = stored.get(file_id)
        if info is None:
            return error(404, "not_found", "file not found")
        etag = f'"{file_id}"'
        if request.headers.get("if-none-match") == etag:
            return Response(status_code=304, headers={"ETag": etag})

        def chunks() -> Iterator[bytes]:
            with (storage_dir / file_id).open("rb") as fh:
                while chunk := fh.read(CHUNK_SIZE):
                    yield chunk

        return StreamingResponse(
            chunks(),
            media_type=info.content_type,
            headers={
                "Content-Length": str(info.size),
                "ETag": etag,
                "Content-Disposition": f'attachment; filename="{info.name}"',
            },
        )

    return app
