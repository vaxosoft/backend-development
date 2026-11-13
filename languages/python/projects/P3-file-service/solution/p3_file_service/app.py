from dataclasses import dataclass
from pathlib import Path
from urllib.parse import quote

from fastapi import FastAPI, Request, Response
from fastapi.responses import FileResponse
from starlette.exceptions import HTTPException

from .errors import register_error_handlers
from .multipart_stream import MalformedUpload, open_file_part
from .storage import DEFAULT_MAX_SIZE, FileStore, StoredFile


@dataclass(slots=True, frozen=True)
class Config:
    storage_dir: Path = Path("uploads")
    max_size: int = DEFAULT_MAX_SIZE


def content_disposition(name: str) -> str:
    """Header value: attachment; filename="<ascii name>" (+ filename*= for non-ASCII names)."""
    ascii_name = name.encode("ascii", "replace").decode("ascii").replace("?", "_")
    header = f'attachment; filename="{ascii_name}"'
    if ascii_name != name:
        header += f"; filename*=UTF-8''{quote(name)}"
    return header


def etag_matches(if_none_match: str | None, etag: str) -> bool:
    """True when an If-None-Match header value (list, `*`, weak `W/` validators) matches etag."""
    if if_none_match is None:
        return False
    candidates = [part.strip() for part in if_none_match.split(",")]
    return "*" in candidates or any(c.removeprefix("W/") == etag for c in candidates)


def create_app(config: Config | None = None) -> FastAPI:
    config = config or Config()
    store = FileStore(config.storage_dir, config.max_size)
    app = FastAPI(title="P3 File Service")
    register_error_handlers(app)

    @app.post("/files", status_code=201)
    async def upload(request: Request) -> StoredFile:
        part = await open_file_part(request, "file")
        if part is None:
            raise MalformedUpload("missing multipart file field 'file'")
        return await store.save(part.chunks(), part.filename, part.content_type)

    @app.get("/files/{file_id}")
    def download(file_id: str, request: Request) -> Response:
        stored = store.get(file_id)
        if stored is None:
            raise HTTPException(status_code=404, detail="file not found")
        etag = f'"{stored.sha256}"'
        if etag_matches(request.headers.get("if-none-match"), etag):
            return Response(status_code=304, headers={"ETag": etag})
        return FileResponse(
            store.data_path(stored.id),
            media_type=stored.content_type,
            headers={"ETag": etag, "Content-Disposition": content_disposition(stored.name)},
        )

    @app.delete("/files/{file_id}", status_code=204)
    def delete(file_id: str) -> None:
        if not store.delete(file_id):
            raise HTTPException(status_code=404, detail="file not found")

    return app
