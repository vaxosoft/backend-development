"""Task Tracker export/import: stream NDJSON through gzip, written atomically."""

import gzip
import hashlib
import json
import os
import tempfile
from collections import Counter
from collections.abc import Iterable, Iterator
from contextlib import contextmanager
from pathlib import Path
from typing import BinaryIO

STATUSES = ("todo", "in_progress", "done")


def generate_tasks(count: int) -> Iterator[dict[str, object]]:
    for n in range(1, count + 1):
        yield {
            "id": n,
            "title": f"Task {n}",
            "status": STATUSES[n % 3],
            "tags": ["export"] if n % 2 else [],
        }


@contextmanager
def atomic_write(target: Path) -> Iterator[BinaryIO]:
    """Write to a temp file next to `target`, then rename it into place.

    Readers see either the old file or the complete new one, never a partial file.
    """
    fd, temp_name = tempfile.mkstemp(dir=target.parent, prefix=f".{target.name}.")
    temp = Path(temp_name)
    try:
        with os.fdopen(fd, "wb") as handle:
            yield handle
        os.replace(temp, target)  # atomic on the same filesystem
    except BaseException:
        temp.unlink(missing_ok=True)
        raise


def export_tasks(path: Path, tasks: Iterable[dict[str, object]]) -> int:
    count = 0
    with atomic_write(path) as raw, gzip.open(raw, "wt", encoding="utf-8") as out:
        for task in tasks:  # one task in memory at a time
            out.write(json.dumps(task, separators=(",", ":")) + "\n")
            count += 1
    return count


def import_tasks(path: Path) -> Iterator[dict[str, object]]:
    with gzip.open(path, "rt", encoding="utf-8") as lines:
        for line in lines:
            yield json.loads(line)


def broken_tasks() -> Iterator[dict[str, object]]:
    yield from generate_tasks(3)
    raise RuntimeError("database connection lost")


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        target = Path(tmp) / "tasks.ndjson.gz"

        written = export_tasks(target, generate_tasks(10_000))
        print("exported:", written)

        by_status = Counter(task["status"] for task in import_tasks(target))
        print("by status:", dict(sorted(by_status.items())))

        with gzip.open(target, "rb") as stream:
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
        print("sha256 of the uncompressed export:", digest[:16])

        before = target.read_bytes()
        try:
            export_tasks(target, broken_tasks())
        except RuntimeError as exc:
            print("second export failed:", exc)
        print("old export untouched:", target.read_bytes() == before)
        print("directory contents:", sorted(p.name for p in Path(tmp).iterdir()))


if __name__ == "__main__":
    main()
