"""Repository pattern over sqlite3: the rest of the app sees Task objects, never SQL or rows."""

import sqlite3
from collections.abc import Callable
from contextlib import closing
from dataclasses import dataclass
from datetime import UTC, date, datetime
from typing import Protocol

SCHEMA = """
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
    due_date TEXT,
    owner_id INTEGER,
    created_at TEXT NOT NULL
);
CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (task_id, tag)
);
CREATE INDEX idx_tasks_status ON tasks(status);
"""


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    due_date: date | None
    tags: tuple[str, ...]
    owner_id: int | None
    created_at: datetime


class TaskRepository(Protocol):
    def add(self, title: str, tags: list[str], due_date: date | None = None) -> Task: ...
    def get(self, task_id: int) -> Task | None: ...
    def list_tasks(
        self, status: str | None = None, limit: int = 20, offset: int = 0
    ) -> tuple[list[Task], int]: ...
    def delete(self, task_id: int) -> bool: ...


class SqliteTaskRepository:
    def __init__(self, conn: sqlite3.Connection, clock: Callable[[], datetime]) -> None:
        conn.row_factory = sqlite3.Row
        conn.execute("PRAGMA foreign_keys = ON")
        self._conn = conn
        self._clock = clock

    def add(self, title: str, tags: list[str], due_date: date | None = None) -> Task:
        with self._conn:  # one transaction for the task and its tags
            cur = self._conn.execute(
                "INSERT INTO tasks (title, due_date, created_at) VALUES (?, ?, ?)",
                (title, due_date.isoformat() if due_date else None, self._clock().isoformat()),
            )
            task_id = cur.lastrowid
            assert task_id is not None
            self._conn.executemany(
                "INSERT INTO task_tags (task_id, tag) VALUES (?, ?)", [(task_id, t) for t in tags]
            )
        created = self.get(task_id)
        assert created is not None
        return created

    def get(self, task_id: int) -> Task | None:
        row = self._conn.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        return self._to_tasks([row])[0] if row else None

    def list_tasks(
        self, status: str | None = None, limit: int = 20, offset: int = 0
    ) -> tuple[list[Task], int]:
        where, params = ("WHERE status = ?", [status]) if status else ("", [])
        # `where` is built only from constant fragments; the values travel as parameters.
        total = self._conn.execute(f"SELECT count(*) FROM tasks {where}", params).fetchone()[0]
        rows = self._conn.execute(
            f"SELECT * FROM tasks {where} ORDER BY id LIMIT ? OFFSET ?", [*params, limit, offset]
        ).fetchall()
        return self._to_tasks(rows), total

    def delete(self, task_id: int) -> bool:
        with self._conn:
            return self._conn.execute("DELETE FROM tasks WHERE id = ?", (task_id,)).rowcount == 1

    def _to_tasks(self, rows: list[sqlite3.Row]) -> list[Task]:
        """Map rows to Tasks, loading all tags with one query instead of one per task."""
        ids = [r["id"] for r in rows]
        tags: dict[int, list[str]] = {i: [] for i in ids}
        placeholders = ",".join("?" * len(ids))
        for tag_row in self._conn.execute(
            f"SELECT task_id, tag FROM task_tags WHERE task_id IN ({placeholders}) ORDER BY tag",
            ids,
        ):
            tags[tag_row["task_id"]].append(tag_row["tag"])
        return [
            Task(
                id=r["id"],
                title=r["title"],
                status=r["status"],
                due_date=date.fromisoformat(r["due_date"]) if r["due_date"] else None,
                tags=tuple(tags[r["id"]]),
                owner_id=r["owner_id"],
                created_at=datetime.fromisoformat(r["created_at"]),
            )
            for r in rows
        ]


def fixed_clock() -> datetime:
    return datetime(2030, 1, 1, 9, 0, tzinfo=UTC)


with closing(sqlite3.connect(":memory:")) as conn:
    conn.executescript(SCHEMA)
    repo: TaskRepository = SqliteTaskRepository(conn, fixed_clock)

    first = repo.add("Write docs", ["docs", "p1"], date(2030, 1, 31))
    print(first)
    repo.add("Review PR", [])
    repo.add("Deploy", ["ops"])
    conn.execute("UPDATE tasks SET status = 'done' WHERE id = 3")

    items, total = repo.list_tasks(limit=2)
    print([t.title for t in items], "of", total)
    items, total = repo.list_tasks(status="done")
    print([t.title for t in items], "of", total)
    print(repo.get(99))
    print(repo.delete(1), repo.delete(1))
    remaining = repo.list_tasks()[1]
    print(
        remaining, "left; tags left:", conn.execute("SELECT count(*) FROM task_tags").fetchone()[0]
    )
