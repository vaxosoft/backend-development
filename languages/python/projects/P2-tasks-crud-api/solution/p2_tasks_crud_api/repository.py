import json
import sqlite3
import threading
from collections.abc import Callable
from datetime import UTC, datetime
from typing import Protocol

from .models import Sort, Task, TaskIn, TaskStatus

Clock = Callable[[], datetime]

SCHEMA = """
CREATE TABLE IF NOT EXISTS tasks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    status     TEXT NOT NULL,
    due_date   TEXT,
    tags       TEXT NOT NULL,   -- JSON array, filtered with json_each()
    owner_id   INTEGER,
    created_at TEXT NOT NULL
)
"""

# User input never reaches the SQL text: `sort` selects one of these fixed clauses.
ORDER_BY: dict[str, str] = {
    "created_at": "created_at ASC, id ASC",
    "-created_at": "created_at DESC, id DESC",
    "due_date": "due_date IS NULL, due_date ASC, id ASC",
    "-due_date": "due_date IS NULL, due_date DESC, id DESC",
}


def utc_now() -> datetime:
    return datetime.now(UTC)


class TaskRepository(Protocol):
    def create(self, data: TaskIn) -> Task: ...

    def get(self, task_id: int) -> Task | None: ...

    def replace(self, task_id: int, data: TaskIn) -> Task | None:
        """Overwrite every writable field; keep id and created_at. None if the id is unknown."""
        ...

    def delete(self, task_id: int) -> bool: ...

    def list(
        self, *, status: TaskStatus | None, tag: str | None, sort: Sort, limit: int, offset: int
    ) -> tuple[list[Task], int]:
        """One page of tasks plus the total number of tasks matching the filters."""
        ...


class SqliteTaskRepository:
    def __init__(self, path: str = ":memory:", clock: Clock = utc_now) -> None:
        self._clock = clock
        # Handlers run in a thread pool, so the one connection is shared and guarded by a lock.
        self._lock = threading.Lock()
        self._db = sqlite3.connect(path, check_same_thread=False)
        self._db.row_factory = sqlite3.Row
        with self._lock, self._db:
            self._db.execute(SCHEMA)

    def close(self) -> None:
        self._db.close()

    def create(self, data: TaskIn) -> Task:
        created_at = self._clock().astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")
        with self._lock, self._db:
            cursor = self._db.execute(
                "INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at)"
                " VALUES (?, ?, ?, ?, ?, ?)",
                (*self._writable(data), created_at),
            )
            row = self._db.execute("SELECT * FROM tasks WHERE id = ?", (cursor.lastrowid,))
            return self._to_task(row.fetchone())

    def get(self, task_id: int) -> Task | None:
        with self._lock:
            row = self._db.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        return self._to_task(row) if row else None

    def replace(self, task_id: int, data: TaskIn) -> Task | None:
        with self._lock, self._db:
            cursor = self._db.execute(
                "UPDATE tasks SET title = ?, status = ?, due_date = ?, tags = ?, owner_id = ?"
                " WHERE id = ?",
                (*self._writable(data), task_id),
            )
            if cursor.rowcount == 0:
                return None
            row = self._db.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
            return self._to_task(row)

    def delete(self, task_id: int) -> bool:
        with self._lock, self._db:
            return self._db.execute("DELETE FROM tasks WHERE id = ?", (task_id,)).rowcount > 0

    def list(
        self, *, status: TaskStatus | None, tag: str | None, sort: Sort, limit: int, offset: int
    ) -> tuple[list[Task], int]:
        conditions: list[str] = []
        params: list[object] = []
        if status is not None:
            conditions.append("status = ?")
            params.append(status.value)
        if tag is not None:
            conditions.append("EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)")
            params.append(tag)
        where = f" WHERE {' AND '.join(conditions)}" if conditions else ""
        with self._lock:
            total = self._db.execute(f"SELECT COUNT(*) FROM tasks{where}", params).fetchone()[0]
            rows = self._db.execute(
                f"SELECT * FROM tasks{where} ORDER BY {ORDER_BY[sort]} LIMIT ? OFFSET ?",
                [*params, limit, offset],
            ).fetchall()
        return [self._to_task(row) for row in rows], total

    @staticmethod
    def _writable(data: TaskIn) -> tuple[str, str, str | None, str, int | None]:
        due = data.due_date.isoformat() if data.due_date else None
        return data.title, data.status.value, due, json.dumps(data.tags), data.owner_id

    @staticmethod
    def _to_task(row: sqlite3.Row) -> Task:
        return Task(
            id=row["id"],
            title=row["title"],
            status=row["status"],
            due_date=row["due_date"],
            tags=json.loads(row["tags"]),
            owner_id=row["owner_id"],
            created_at=datetime.fromisoformat(row["created_at"]),
        )
