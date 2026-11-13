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
        # TODO: INSERT with bound parameters (tags as json.dumps, due_date as ISO text,
        # created_at from the injected clock as "%Y-%m-%dT%H:%M:%SZ"), then return the new Task.
        raise NotImplementedError

    def get(self, task_id: int) -> Task | None:
        # TODO: SELECT by id; None when no row.
        raise NotImplementedError

    def replace(self, task_id: int, data: TaskIn) -> Task | None:
        # TODO: UPDATE ... WHERE id = ?; use cursor.rowcount to detect an unknown id.
        raise NotImplementedError

    def delete(self, task_id: int) -> bool:
        # TODO: DELETE ... WHERE id = ?
        raise NotImplementedError

    def list(
        self, *, status: TaskStatus | None, tag: str | None, sort: Sort, limit: int, offset: int
    ) -> tuple[list[Task], int]:
        # TODO: build WHERE from the optional filters (bound parameters only; a tag matches with
        # EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)), map `sort` through a
        # fixed dict of ORDER BY clauses (never format user input into SQL), count the total
        # without LIMIT, then LIMIT/OFFSET.
        raise NotImplementedError

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
