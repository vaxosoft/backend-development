"""Exercise 2 (Medium): copy a stream in bounded memory while hashing it and enforcing a limit."""

from dataclasses import dataclass
from typing import Protocol


class Readable(Protocol):
    def read(self, size: int, /) -> bytes: ...


class Writable(Protocol):
    def write(self, data: bytes, /) -> int: ...


class TooLargeError(Exception):
    pass


@dataclass(slots=True, frozen=True)
class CopyResult:
    size: int
    sha256: str


def copy_and_hash(
    src: Readable, dst: Writable, *, chunk_size: int = 64 * 1024, max_bytes: int | None = None
) -> CopyResult:
    """Copy `src` to `dst`, returning the byte count and the SHA-256 hex digest.

    - Read with `src.read(chunk_size)` only: never read() everything at once. A read may
      return fewer bytes than requested; an empty result means end of stream.
    - Write every chunk to `dst` as it arrives.
    - If more than `max_bytes` bytes arrive, raise TooLargeError as soon as the limit is
      crossed: do not write the chunk that crosses it and do not read any further.
      Exactly `max_bytes` bytes is allowed. `None` means unlimited.
    """
    raise NotImplementedError("TODO: implement copy_and_hash")
