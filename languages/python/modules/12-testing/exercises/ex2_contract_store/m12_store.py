"""Exercise 2: an in-memory store that honours the same contract as the SQLite one.

`SqliteTaskStore` is complete and is the reference behaviour. Implement `InMemoryTaskStore`
so that the contract tests (which run against BOTH) pass:

* `add(title, tags=())` -> Task with the next id (1, 2, 3...), status "todo", tags as a tuple
* `get(task_id)` -> Task or None
* `list_tasks(status=None, limit=20, offset=0)` -> new list ordered by id; filter by status;
  an unknown status raises ValueError
* `set_status(task_id, status)` -> updated Task, or None for an unknown id; an invalid
  status raises ValueError (even when the id is unknown)
* `delete(task_id)` -> True if something was deleted, else False. Ids are NEVER reused.
"""

import sqlite3
from collections.abc import Sequence
from dataclasses import dataclass
from typing import Protocol

STATUSES = ("todo", "in_progress", "done")


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    tags: tuple[str, ...]


class TaskStore(Protocol):
    def add(self, title: str, tags: Sequence[str] = ()) -> Task: ...
    def get(self, task_id: int) -> Task | None: ...
    def list_tasks(
        self, status: str | None = None, limit: int = 20, offset: int = 0
    ) -> list[Task]: ...
    def set_status(self, task_id: int, status: str) -> Task | None: ...
    def delete(self, task_id: int) -> bool: ...


def check_status(status: str) -> str:
    if status not in STATUSES:
        raise ValueError(f"invalid status {status!r}")
    return status


class SqliteTaskStore:
    def __init__(self) -> None:
        self._db = sqlite3.connect(":memory:")
        self._db.execute(
            "CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT,"
            " title TEXT NOT NULL, status TEXT NOT NULL, tags TEXT NOT NULL)"
        )

    @staticmethod
    def _task(row: tuple[int, str, str, str]) -> Task:
        task_id, title, status, tags = row
        return Task(task_id, title, status, tuple(tags.split(",")) if tags else ())

    def add(self, title: str, tags: Sequence[str] = ()) -> Task:
        cursor = self._db.execute(
            "INSERT INTO tasks (title, status, tags) VALUES (?, 'todo', ?)", (title, ",".join(tags))
        )
        task = self.get(cursor.lastrowid or 0)
        assert task is not None
        return task

    def get(self, task_id: int) -> Task | None:
        row = self._db.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        return self._task(row) if row else None

    def list_tasks(self, status: str | None = None, limit: int = 20, offset: int = 0) -> list[Task]:
        if status is None:
            rows = self._db.execute(
                "SELECT * FROM tasks ORDER BY id LIMIT ? OFFSET ?", (limit, offset)
            )
        else:
            rows = self._db.execute(
                "SELECT * FROM tasks WHERE status = ? ORDER BY id LIMIT ? OFFSET ?",
                (check_status(status), limit, offset),
            )
        return [self._task(row) for row in rows]

    def set_status(self, task_id: int, status: str) -> Task | None:
        self._db.execute(
            "UPDATE tasks SET status = ? WHERE id = ?", (check_status(status), task_id)
        )
        return self.get(task_id)

    def delete(self, task_id: int) -> bool:
        return self._db.execute("DELETE FROM tasks WHERE id = ?", (task_id,)).rowcount > 0


class InMemoryTaskStore:
    def __init__(self) -> None:
        pass

    def add(self, title: str, tags: Sequence[str] = ()) -> Task:
        raise NotImplementedError

    def get(self, task_id: int) -> Task | None:
        raise NotImplementedError

    def list_tasks(self, status: str | None = None, limit: int = 20, offset: int = 0) -> list[Task]:
        raise NotImplementedError

    def set_status(self, task_id: int, status: str) -> Task | None:
        raise NotImplementedError

    def delete(self, task_id: int) -> bool:
        raise NotImplementedError
