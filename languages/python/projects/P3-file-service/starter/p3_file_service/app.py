from dataclasses import dataclass
from pathlib import Path

from fastapi import FastAPI, Request, Response

from .errors import register_error_handlers
from .storage import DEFAULT_MAX_SIZE, FileStore, StoredFile


@dataclass(slots=True, frozen=True)
class Config:
    storage_dir: Path = Path("uploads")
    max_size: int = DEFAULT_MAX_SIZE


def content_disposition(name: str) -> str:
    """Header value: attachment; filename="<ascii name>" (+ filename*= for non-ASCII names)."""
    # TODO: the name is already sanitized (no quotes or control characters). Replace
    # non-ASCII characters in the plain filename and add RFC 5987 `filename*=UTF-8''...`
    # (urllib.parse.quote) when the name is not pure ASCII.
    return f'attachment; filename="{name}"'


def etag_matches(if_none_match: str | None, etag: str) -> bool:
    """True when an If-None-Match header value (list, `*`, weak `W/` validators) matches etag."""
    # TODO
    return False


def create_app(config: Config | None = None) -> FastAPI:
    config = config or Config()
    store = FileStore(config.storage_dir, config.max_size)
    app = FastAPI(title="P3 File Service")
    register_error_handlers(app)

    @app.post("/files", status_code=201)
    async def upload(request: Request) -> StoredFile:
        # TODO: part = await open_file_part(request, "file")  (import it from .multipart_stream;
        # None means no such part -> raise MalformedUpload), then
        # `await store.save(part.chunks(), part.filename, part.content_type)`.
        raise NotImplementedError

    @app.get("/files/{file_id}")
    def download(file_id: str, request: Request) -> Response:
        # TODO: 404 (HTTPException) when unknown. Headers: ETag: "<sha256>", Content-Disposition.
        # If-None-Match matching the ETag -> 304 without body; otherwise
        # fastapi.responses.FileResponse(store.data_path(id), media_type=..., headers=...)
        # streams the file with Content-Length.
        raise NotImplementedError

    @app.delete("/files/{file_id}", status_code=204)
    def delete(file_id: str) -> None:
        # TODO: 404 (HTTPException) when store.delete() says the id is unknown.
        store.delete(file_id)

    return app
