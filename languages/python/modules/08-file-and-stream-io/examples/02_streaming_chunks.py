"""Streams: read in chunks, hash while reading, copy between file-like objects."""

import hashlib
import io
import shutil
import tempfile
from pathlib import Path
from typing import BinaryIO, TextIO

CHUNK = 64 * 1024


def sha256_of(stream: BinaryIO) -> str:
    digest = hashlib.sha256()
    while chunk := stream.read(CHUNK):
        digest.update(chunk)
    return digest.hexdigest()


def count_lines(stream: TextIO) -> int:
    return sum(1 for _ in stream)  # iterating a text file yields one line at a time


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        source = Path(tmp) / "big.bin"
        source.write_bytes(b"0123456789abcdef" * 65536)  # 1 MiB

        # Never source.read_bytes() for data of unknown size: read a chunk at a time
        with source.open("rb") as handle:
            print("sha256 by chunks:  ", sha256_of(handle)[:16])
        with source.open("rb") as handle:
            print("hashlib.file_digest:", hashlib.file_digest(handle, "sha256").hexdigest()[:16])

        # copyfileobj moves data between any two file-like objects with bounded memory
        copy = Path(tmp) / "copy.bin"
        with source.open("rb") as reader, copy.open("wb") as writer:
            shutil.copyfileobj(reader, writer, length=CHUNK)
        print("copy size:", copy.stat().st_size)

        # io.BytesIO / io.StringIO are in-memory files, ideal for tests and small payloads
        print("in-memory:", sha256_of(io.BytesIO(b"hello"))[:16])
        print("lines in a StringIO:", count_lines(io.StringIO("a\nb\nc\n")))

        log = Path(tmp) / "app.log"
        log.write_text("start\nwork\nstop\n", encoding="utf-8")
        with log.open(encoding="utf-8") as text_handle:
            print("lines in a file:", count_lines(text_handle))

        # After the with block the handle is closed; using it again fails loudly
        try:
            text_handle.read()
        except ValueError as exc:
            print("closed file:", exc)


if __name__ == "__main__":
    main()
