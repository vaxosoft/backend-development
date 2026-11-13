"""Exercise 3: stream uploads to disk and downloads from disk without buffering whole files."""

from pathlib import Path

from fastapi import FastAPI

CHUNK_SIZE = 64 * 1024


def sanitize_filename(raw: str | None) -> str:
    """Reduce a client-supplied filename to a harmless display name.

    - keep only the last path component (both "/" and "\\" separate components)
    - replace every character that is not a word character, ".", "-" or space with "_"
    - strip leading dots (no hidden files, no "..")
    - fall back to "upload.bin" when nothing is left (or `raw` is None)
    """
    raise NotImplementedError


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
    return app
