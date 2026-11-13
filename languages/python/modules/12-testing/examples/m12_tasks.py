"""The code under test for the examples of module 12: a tiny Task Tracker core."""

import sqlite3
from collections.abc import Callable, Iterable
from dataclasses import dataclass, replace
from datetime import UTC, datetime
from typing import Protocol

STATUSES = ("todo", "in_progress", "done")

type Clock = Callable[[], datetime]


class ValidationError(ValueError):
    pass


class NotFoundError(LookupError):
    pass


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    tags: tuple[str, ...]
    created_at: datetime


def validate_title(title: str) -> str:
    cleaned = title.strip()
    if not 1 <= len(cleaned) <= 200:
        raise ValidationError("title must be 1-200 characters")
    return cleaned


def normalize_tags(tags: Iterable[str]) -> list[str]:
    """Lower-case, trim, drop blanks and duplicates (keeping first-seen order); max 10."""
    result: list[str] = []
    for tag in tags:
        cleaned = tag.strip().lower()
        if cleaned and cleaned not in result:
            result.append(cleaned)
    if len(result) > 10:
        raise ValidationError("at most 10 tags")
    return result


class TaskRepository(Protocol):
    def add(self, title: str, tags: tuple[str, ...], created_at: datetime) -> Task: ...
    def get(self, task_id: int) -> Task | None: ...
    def list_all(self) -> list[Task]: ...
    def set_status(self, task_id: int, status: str) -> Task | None: ...


class InMemoryTaskRepository:
    def __init__(self) -> None:
        self._tasks: dict[int, Task] = {}

    def add(self, title: str, tags: tuple[str, ...], created_at: datetime) -> Task:
        task = Task(len(self._tasks) + 1, title, "todo", tags, created_at)
        self._tasks[task.id] = task
        return task

    def get(self, task_id: int) -> Task | None:
        return self._tasks.get(task_id)

    def list_all(self) -> list[Task]:
        return list(self._tasks.values())

    def set_status(self, task_id: int, status: str) -> Task | None:
        task = self._tasks.get(task_id)
        if task is None:
            return None
        self._tasks[task_id] = updated = replace(task, status=status)
        return updated


class SqliteTaskRepository:
    def __init__(self, path: str = ":memory:") -> None:
        # FastAPI runs sync handlers in a thread pool, so the connection must be usable there.
        self._db = sqlite3.connect(path, check_same_thread=False)
        self._db.execute(
            "CREATE TABLE IF NOT EXISTS tasks (id INTEGER PRIMARY KEY AUTOINCREMENT,"
            " title TEXT NOT NULL, status TEXT NOT NULL, tags TEXT NOT NULL,"
            " created_at TEXT NOT NULL)"
        )

    def close(self) -> None:
        self._db.close()

    @staticmethod
    def _task(row: tuple[int, str, str, str, str]) -> Task:
        task_id, title, status, tags, created_at = row
        return Task(
            task_id,
            title,
            status,
            tuple(tags.split(",")) if tags else (),
            datetime.fromisoformat(created_at),
        )

    def add(self, title: str, tags: tuple[str, ...], created_at: datetime) -> Task:
        with self._db:
            cursor = self._db.execute(
                "INSERT INTO tasks (title, status, tags, created_at) VALUES (?, 'todo', ?, ?)",
                (title, ",".join(tags), created_at.isoformat()),
            )
        assert cursor.lastrowid is not None
        task = self.get(cursor.lastrowid)
        assert task is not None
        return task

    def get(self, task_id: int) -> Task | None:
        row = self._db.execute("SELECT * FROM tasks WHERE id = ?", (task_id,)).fetchone()
        return self._task(row) if row else None

    def list_all(self) -> list[Task]:
        return [self._task(row) for row in self._db.execute("SELECT * FROM tasks ORDER BY id")]

    def set_status(self, task_id: int, status: str) -> Task | None:
        with self._db:
            self._db.execute("UPDATE tasks SET status = ? WHERE id = ?", (status, task_id))
        return self.get(task_id)


class Notifier(Protocol):
    def task_created(self, task: Task) -> None: ...


class TaskService:
    def __init__(
        self,
        repo: TaskRepository,
        clock: Clock = lambda: datetime.now(UTC),
        notifier: Notifier | None = None,
    ) -> None:
        self._repo = repo
        self._clock = clock
        self._notifier = notifier

    def create(self, title: str, tags: Iterable[str] = ()) -> Task:
        task = self._repo.add(validate_title(title), tuple(normalize_tags(tags)), self._clock())
        if self._notifier:
            self._notifier.task_created(task)
        return task

    def get(self, task_id: int) -> Task:
        task = self._repo.get(task_id)
        if task is None:
            raise NotFoundError(f"task {task_id} not found")
        return task

    def list_all(self) -> list[Task]:
        return self._repo.list_all()

    def set_status(self, task_id: int, status: str) -> Task:
        if status not in STATUSES:
            raise ValidationError(f"status must be one of {', '.join(STATUSES)}")
        task = self._repo.set_status(task_id, status)
        if task is None:
            raise NotFoundError(f"task {task_id} not found")
        return task
