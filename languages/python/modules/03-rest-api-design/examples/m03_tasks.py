"""Shared by the module 03 examples: Task models with validation, an in-memory store and a clock."""

import os
from collections.abc import Callable
from datetime import UTC, date, datetime
from enum import StrEnum
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field, StringConstraints, field_validator

Clock = Callable[[], datetime]

Title = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)]
Tag = Annotated[str, StringConstraints(min_length=1, max_length=30)]
SortKey = Literal["created_at", "-created_at", "due_date", "-due_date"]

DEFAULT_LIMIT = 20
MAX_LIMIT = 100


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


def check_tags(tags: list[str] | None) -> list[str] | None:
    if tags is None:
        raise ValueError("must not be null")
    if len(tags) > 10:
        raise ValueError("at most 10 tags")
    if any(tag != tag.lower() for tag in tags):
        raise ValueError("tags must be lowercase")
    if len(set(tags)) != len(tags):
        raise ValueError("tags must be unique")
    return tags


class TaskCreate(BaseModel):
    """Body of POST and PUT. Unknown fields are rejected instead of silently ignored."""

    model_config = ConfigDict(extra="forbid")

    title: Title
    status: Status = Status.TODO
    due_date: date | None = None
    tags: list[Tag] = Field(default_factory=list)
    owner_id: int | None = None

    _tags = field_validator("tags")(check_tags)


class TaskPatch(BaseModel):
    """Body of PATCH: absent means unchanged, an explicit null is allowed only for due_date."""

    model_config = ConfigDict(extra="forbid")

    title: Title | None = None
    status: Status | None = None
    due_date: date | None = None
    tags: list[Tag] | None = None

    @field_validator("title", "status", "tags", mode="before")
    @classmethod
    def not_null(cls, value: object) -> object:
        if value is None:
            raise ValueError("must not be null")
        return value

    _tags = field_validator("tags")(check_tags)


class Task(BaseModel):
    id: int
    title: str
    status: Status
    due_date: date | None
    tags: list[str]
    owner_id: int | None
    created_at: datetime


def utc_now() -> datetime:
    return datetime.now(UTC).replace(microsecond=0)


def clock_from_env() -> Clock:
    """FIXED_NOW=2030-01-01T09:00:00Z freezes the clock, which makes demo output reproducible."""
    fixed = os.environ.get("FIXED_NOW")
    if not fixed:
        return utc_now
    frozen = datetime.fromisoformat(fixed)
    return lambda: frozen


def _due_key(task: Task) -> tuple[bool, date, int]:
    # Tasks without a due date sort last in ascending order; id keeps the order stable.
    return (task.due_date is None, task.due_date or date.max, task.id)


class TaskStore:
    def __init__(self, clock: Clock = utc_now) -> None:
        self._clock = clock
        self._tasks: dict[int, Task] = {}
        self._next_id = 1

    def create(self, data: TaskCreate) -> Task:
        task = Task(id=self._next_id, created_at=self._clock(), **data.model_dump())
        self._tasks[task.id] = task
        self._next_id += 1
        return task

    def get(self, task_id: int) -> Task | None:
        return self._tasks.get(task_id)

    def replace(self, task_id: int, data: TaskCreate) -> Task | None:
        old = self._tasks.get(task_id)
        if old is None:
            return None
        task = Task(id=old.id, created_at=old.created_at, **data.model_dump())
        self._tasks[task_id] = task
        return task

    def patch(self, task_id: int, patch: TaskPatch) -> Task | None:
        old = self._tasks.get(task_id)
        if old is None:
            return None
        task = old.model_copy(update=patch.model_dump(exclude_unset=True))
        self._tasks[task_id] = task
        return task

    def delete(self, task_id: int) -> bool:
        return self._tasks.pop(task_id, None) is not None

    def query(
        self,
        *,
        status: Status | None = None,
        tag: str | None = None,
        sort: SortKey = "-created_at",
        limit: int = DEFAULT_LIMIT,
        offset: int = 0,
    ) -> tuple[list[Task], int]:
        """Return one page and the total number of matches (before pagination)."""
        matches = [
            t
            for t in self._tasks.values()
            if (status is None or t.status is status) and (tag is None or tag in t.tags)
        ]
        field = sort.lstrip("-")
        descending = sort.startswith("-")
        if field == "created_at":
            matches.sort(key=lambda t: (t.created_at, t.id), reverse=descending)
        else:
            matches.sort(key=_due_key)
            if descending:
                dated = [t for t in matches if t.due_date is not None]
                undated = [t for t in matches if t.due_date is None]
                matches = [*reversed(dated), *undated]
        return matches[offset : offset + limit], len(matches)
