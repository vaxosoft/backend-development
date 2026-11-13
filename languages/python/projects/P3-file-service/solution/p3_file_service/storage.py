import hashlib
import json
import re
import unicodedata
import uuid
from collections.abc import AsyncIterator
from dataclasses import asdict, dataclass
from pathlib import Path

ID_PATTERN = re.compile(r"[0-9a-f]{32}")
DEFAULT_MAX_SIZE = 5 * 1024 * 1024


class FileTooLarge(Exception):
    """The upload is bigger than the configured limit."""


@dataclass(slots=True, frozen=True)
class StoredFile:
    id: str
    name: str
    size: int
    sha256: str
    content_type: str


def sanitize_filename(raw: str) -> str:
    """Turn a client-supplied name into a safe display name (never used as a path)."""
    name = re.split(r"[\\/]", raw)[-1]
    name = "".join(ch for ch in name if unicodedata.category(ch) != "Cc")
    name = name.replace('"', "_").strip()[:255]
    return name if name.strip(".") else "file"


class FileStore:
    """Files live in `root` as <id>.bin with a <id>.json sidecar holding the metadata."""

    def __init__(self, root: Path, max_size: int = DEFAULT_MAX_SIZE) -> None:
        self._root = root
        self._max_size = max_size
        root.mkdir(parents=True, exist_ok=True)

    def data_path(self, file_id: str) -> Path:
        return self._root / f"{file_id}.bin"

    def _meta_path(self, file_id: str) -> Path:
        return self._root / f"{file_id}.json"

    async def save(self, chunks: AsyncIterator[bytes], name: str, content_type: str) -> StoredFile:
        """Stream `chunks` to disk under a new random id while counting and hashing them."""
        file_id = uuid.uuid4().hex
        partial = self._root / f"{file_id}.part"
        digest = hashlib.sha256()
        size = 0
        try:
            with partial.open("wb") as out:
                async for chunk in chunks:
                    size += len(chunk)
                    if size > self._max_size:
                        raise FileTooLarge(f"more than {self._max_size} bytes")
                    digest.update(chunk)
                    out.write(chunk)
            stored = StoredFile(
                file_id, sanitize_filename(name), size, digest.hexdigest(), content_type
            )
            partial.replace(self.data_path(file_id))
            self._meta_path(file_id).write_text(json.dumps(asdict(stored)), encoding="utf-8")
        except BaseException:
            partial.unlink(missing_ok=True)
            self.data_path(file_id).unlink(missing_ok=True)
            raise
        return stored

    def get(self, file_id: str) -> StoredFile | None:
        if not ID_PATTERN.fullmatch(file_id):
            return None
        try:
            return StoredFile(**json.loads(self._meta_path(file_id).read_text(encoding="utf-8")))
        except FileNotFoundError:
            return None

    def delete(self, file_id: str) -> bool:
        if self.get(file_id) is None:
            return False
        self._meta_path(file_id).unlink(missing_ok=True)
        self.data_path(file_id).unlink(missing_ok=True)
        return True
