import gzip
from collections.abc import Iterator
from pathlib import Path

import pytest
from m08_ndjson import NdjsonError, read_ndjson_gz, write_ndjson_gz


def gunzip(path: Path) -> str:
    return gzip.decompress(path.read_bytes()).decode("utf-8")


def test_round_trip_and_count(tmp_path: Path) -> None:
    target = tmp_path / "tasks.ndjson.gz"
    records = [{"id": 1, "title": "Write docs"}, {"id": 2, "title": "Ship", "tags": ["a"]}]

    count = write_ndjson_gz(target, records)

    assert count == 2
    assert list(read_ndjson_gz(target)) == records


def test_output_is_compact_utf8_lines(tmp_path: Path) -> None:
    target = tmp_path / "out.gz"

    write_ndjson_gz(target, [{"id": 1, "title": "café"}, {"id": 2, "title": "ok"}])

    assert gunzip(target) == '{"id":1,"title":"café"}\n{"id":2,"title":"ok"}\n'


def test_empty_export_is_a_valid_empty_file(tmp_path: Path) -> None:
    target = tmp_path / "empty.gz"

    assert write_ndjson_gz(target, []) == 0
    assert list(read_ndjson_gz(target)) == []


def test_records_are_consumed_lazily(tmp_path: Path) -> None:
    produced: list[int] = []

    def records() -> Iterator[dict[str, object]]:
        for n in range(3):
            produced.append(n)
            yield {"n": n}

    assert write_ndjson_gz(tmp_path / "lazy.gz", records()) == 3
    assert produced == [0, 1, 2]


def test_failure_keeps_the_previous_file_and_leaves_no_temp_files(tmp_path: Path) -> None:
    target = tmp_path / "tasks.gz"
    write_ndjson_gz(target, [{"id": 1}])
    before = target.read_bytes()

    def broken() -> Iterator[dict[str, object]]:
        yield {"id": 2}
        raise RuntimeError("database connection lost")

    with pytest.raises(RuntimeError, match="connection lost"):
        write_ndjson_gz(target, broken())

    assert target.read_bytes() == before
    assert [p.name for p in tmp_path.iterdir()] == ["tasks.gz"]


def test_failure_on_a_new_target_leaves_nothing(tmp_path: Path) -> None:
    def broken() -> Iterator[dict[str, object]]:
        raise RuntimeError("boom")
        yield {}

    with pytest.raises(RuntimeError):
        write_ndjson_gz(tmp_path / "new.gz", broken())

    assert list(tmp_path.iterdir()) == []


def test_reading_is_lazy_and_reports_the_line_number(tmp_path: Path) -> None:
    target = tmp_path / "bad.gz"
    target.write_bytes(gzip.compress(b'{"id":1}\n\n{"id":2}\n{oops\n{"id":4}\n'))

    records = read_ndjson_gz(target)

    assert next(records) == {"id": 1}
    assert next(records) == {"id": 2}
    with pytest.raises(NdjsonError) as info:
        next(records)
    assert info.value.line_number == 4


@pytest.mark.parametrize("line", [b"[1, 2]", b'"text"', b"42"])
def test_non_object_lines_are_rejected(tmp_path: Path, line: bytes) -> None:
    target = tmp_path / "bad.gz"
    target.write_bytes(gzip.compress(b'{"id":1}\n' + line + b"\n"))

    with pytest.raises(NdjsonError) as info:
        list(read_ndjson_gz(target))

    assert info.value.line_number == 2
