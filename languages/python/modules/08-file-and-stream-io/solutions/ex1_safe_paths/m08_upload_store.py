"""Exercise 1 (Easy): store uploads safely; the client-supplied name is never used as a path."""

import re
import uuid
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path


class UnknownFileError(Exception):
    pass


@dataclass(slots=True, frozen=True)
class StoredFile:
    id: str
    name: str
    size: int


ID_PATTERN = re.compile(r"[0-9a-f]{32}")
CONTROL_CHARS = re.compile(r"[\x00-\x1f\x7f]")


def display_name(filename: str) -> str:
    """Turn a client-supplied filename into a harmless display name.

    - Keep only the last component, splitting on both "/" and "\\".
    - Replace every control character (code point < 32 or == 127) with "_".
    - Strip surrounding whitespace and dots.
    - If nothing is left, return "file".
    """
    leaf = re.split(r"[\\/]", filename)[-1]
    return CONTROL_CHARS.sub("_", leaf).strip().strip(".").strip() or "file"


def new_file_id() -> str:
    return uuid.uuid4().hex


class UploadStore:
    """Files live at `base_dir / <id>`. An id is exactly 32 lowercase hex characters."""

    def __init__(self, base_dir: Path, new_id: Callable[[], str] = new_file_id) -> None:
        self._base_dir = base_dir
        self._new_id = new_id

    def _path_for(self, file_id: str) -> Path:
        if not ID_PATTERN.fullmatch(file_id):
            raise UnknownFileError(file_id)
        path = self._base_dir / file_id
        if not path.is_file():
            raise UnknownFileError(file_id)
        return path

    def save(self, filename: str, data: bytes) -> StoredFile:
        """Write `data` to `base_dir / <generated id>` and return its metadata.

        `name` is display_name(filename). The filename must not influence the path.
        """
        file_id = self._new_id()
        (self._base_dir / file_id).write_bytes(data)
        return StoredFile(file_id, display_name(filename), len(data))

    def read(self, file_id: str) -> bytes:
        """Return the stored bytes. Raise UnknownFileError for malformed or unknown ids."""
        return self._path_for(file_id).read_bytes()

    def delete(self, file_id: str) -> None:
        """Remove a stored file. Raise UnknownFileError for malformed or unknown ids."""
        self._path_for(file_id).unlink()
