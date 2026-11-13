import csv
import io
from dataclasses import dataclass
from datetime import date

COLUMNS = ["id", "title", "status", "due_date", "tags"]


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    due_date: date | None
    tags: tuple[str, ...]


def tasks_to_csv(tasks: list[Task]) -> str:
    out = io.StringIO(newline="")
    writer = csv.writer(out, lineterminator="\n")
    writer.writerow(COLUMNS)
    for t in tasks:
        due = t.due_date.isoformat() if t.due_date else ""
        writer.writerow([t.id, t.title, t.status, due, ";".join(t.tags)])
    return out.getvalue()


def tasks_from_csv(text: str) -> list[Task]:
    reader = csv.DictReader(io.StringIO(text, newline=""))
    missing = [c for c in COLUMNS if c not in (reader.fieldnames or [])]
    if missing:
        raise ValueError(f"missing column: {', '.join(missing)}")
    return [
        Task(
            id=int(row["id"]),
            title=row["title"],
            status=row["status"],
            due_date=date.fromisoformat(row["due_date"]) if row["due_date"] else None,
            tags=tuple(row["tags"].split(";")) if row["tags"] else (),
        )
        for row in reader
    ]
