import re
from collections.abc import AsyncIterator
from dataclasses import dataclass
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
    # TODO: keep only the part after the last "/" or "\", drop control characters
    # (unicodedata.category == "Cc"), replace double quotes, trim whitespace, limit to
    # 255 characters, and never return an empty name or one made only of dots ("file" is fine).
    return raw


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
        # TODO:
        # 1. id = uuid.uuid4().hex; write to "<id>.part" chunk by chunk (never join the chunks);
        # 2. keep a running size and hashlib.sha256; raise FileTooLarge as soon as size > max;
        # 3. on success rename .part to .bin (Path.replace) and write the .json sidecar last;
        # 4. on ANY failure (also cancellation) remove the partial files and re-raise.
        raise NotImplementedError

    def get(self, file_id: str) -> StoredFile | None:
        # TODO: reject ids that do not match ID_PATTERN *before* touching the filesystem,
        # then read the sidecar; None when it does not exist.
        raise NotImplementedError

    def delete(self, file_id: str) -> bool:
        # TODO: same id check; remove the data file and the sidecar; False when unknown.
        raise NotImplementedError
