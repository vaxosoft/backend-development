"""Exercise 3 (Stretch): a gzip NDJSON export written atomically and read lazily."""

from collections.abc import Iterable, Iterator, Mapping
from pathlib import Path


class NdjsonError(ValueError):
    def __init__(self, line_number: int, reason: str) -> None:
        super().__init__(f"line {line_number}: {reason}")
        self.line_number = line_number


def write_ndjson_gz(path: Path, records: Iterable[Mapping[str, object]]) -> int:
    """Write one compact JSON object per line, gzip-compressed, and return the count.

    - Lines are UTF-8, use separators (",", ":") and keep non-ASCII text as is
      (`ensure_ascii=False`), each followed by "\\n".
    - Consume `records` one at a time; do not build the whole output in memory.
    - Atomic: write to a temporary file in the same directory, then os.replace() it onto
      `path`. If anything fails (for example `records` raises), an existing `path` keeps
      its old content and no temporary file is left behind.
    """
    raise NotImplementedError("TODO: implement write_ndjson_gz")


def read_ndjson_gz(path: Path) -> Iterator[dict[str, object]]:
    """Yield the objects of a file written by write_ndjson_gz, one line at a time.

    - Lazy: a malformed line only fails when the iteration reaches it.
    - Blank lines are skipped but still count for line numbers (1-based).
    - A line that is not valid JSON, or not a JSON object, raises NdjsonError(line_number, ...).
    """
    raise NotImplementedError("TODO: implement read_ndjson_gz")
