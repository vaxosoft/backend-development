import threading
from collections.abc import Callable
from datetime import UTC, datetime
from typing import Protocol

from .models import Task, TaskCreate


class TaskStore(Protocol):
    """What the HTTP layer needs from storage. Tests can pass any object with these methods."""

    def add(self, data: TaskCreate) -> Task: ...

    def get(self, task_id: int) -> Task | None: ...

    def list_all(self) -> list[Task]: ...


def utc_now() -> datetime:
    return datetime.now(UTC)


class InMemoryTaskStore:
    def __init__(self, clock: Callable[[], datetime] = utc_now) -> None:
        self._clock = clock
        self._tasks: dict[int, Task] = {}
        self._lock = threading.Lock()  # sync endpoints run in a thread pool

    def add(self, data: TaskCreate) -> Task:
        with self._lock:
            task = Task(id=len(self._tasks) + 1, created_at=self._clock(), **data.model_dump())
            self._tasks[task.id] = task
            return task

    def get(self, task_id: int) -> Task | None:
        with self._lock:
            return self._tasks.get(task_id)

    def list_all(self) -> list[Task]:
        with self._lock:
            return list(self._tasks.values())
