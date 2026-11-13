import hashlib
import io

import pytest
from m08_copy_hash import CopyResult, TooLargeError, copy_and_hash


class RecordingReader:
    """Serves `data` in pieces of at most `max_piece` bytes and records each requested size."""

    def __init__(self, data: bytes, max_piece: int | None = None) -> None:
        self._data = io.BytesIO(data)
        self._max_piece = max_piece
        self.requests: list[int | None] = []

    def read(self, size: int | None = -1) -> bytes:
        self.requests.append(size)
        if size is None or size < 0:
            raise AssertionError("read() without a size loads everything into memory")
        if self._max_piece is not None:
            size = min(size, self._max_piece)
        return self._data.read(size)


class EndlessZeros:
    def __init__(self) -> None:
        self.delivered = 0

    def read(self, size: int | None = -1) -> bytes:
        assert size is not None
        assert size > 0
        self.delivered += size
        return bytes(size)


def sha(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def test_copies_bytes_and_reports_size_and_digest() -> None:
    data = bytes(range(256)) * 100
    dst = io.BytesIO()

    result = copy_and_hash(io.BytesIO(data), dst, chunk_size=1000)

    assert dst.getvalue() == data
    assert result == CopyResult(len(data), sha(data))


def test_empty_stream() -> None:
    dst = io.BytesIO()

    result = copy_and_hash(io.BytesIO(b""), dst)

    assert result == CopyResult(0, sha(b""))
    assert dst.getvalue() == b""


def test_reads_in_bounded_chunks() -> None:
    reader = RecordingReader(b"x" * 1000)

    copy_and_hash(reader, io.BytesIO(), chunk_size=64)

    assert reader.requests
    assert all(size is not None and size <= 64 for size in reader.requests)


def test_short_reads_do_not_end_the_copy_early() -> None:
    data = b"abcdefghij" * 10
    dst = io.BytesIO()

    result = copy_and_hash(RecordingReader(data, max_piece=3), dst, chunk_size=64)

    assert dst.getvalue() == data
    assert result == CopyResult(100, sha(data))


def test_exactly_max_bytes_is_allowed() -> None:
    dst = io.BytesIO()

    result = copy_and_hash(io.BytesIO(b"x" * 100), dst, chunk_size=30, max_bytes=100)

    assert result.size == 100


def test_one_byte_over_the_limit_raises_and_keeps_dst_within_the_limit() -> None:
    dst = io.BytesIO()

    with pytest.raises(TooLargeError):
        copy_and_hash(io.BytesIO(b"x" * 101), dst, chunk_size=30, max_bytes=100)

    assert len(dst.getvalue()) <= 100


def test_limit_stops_reading_an_endless_source() -> None:
    source = EndlessZeros()
    dst = io.BytesIO()

    with pytest.raises(TooLargeError):
        copy_and_hash(source, dst, chunk_size=1024, max_bytes=5000)

    assert source.delivered <= 5000 + 1024
    assert len(dst.getvalue()) <= 5000
