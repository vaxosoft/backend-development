"""Task Tracker domain in plain Python: model, validation, repository protocol, queries."""

from collections.abc import Iterable, Iterator
from dataclasses import dataclass, field, replace
from datetime import UTC, date, datetime
from enum import StrEnum
from typing import Protocol


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class ValidationError(Exception):
    def __init__(self, fields: dict[str, str]) -> None:
        super().__init__(f"invalid task: {fields}")
        self.fields = fields


@dataclass(slots=True, frozen=True)
class Task:
    id: int
    title: str
    status: Status = Status.TODO
    due_date: date | None = None
    tags: tuple[str, ...] = ()
    owner_id: int | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


def validate(title: str, tags: Iterable[str] = ()) -> tuple[str, tuple[str, ...]]:
    """Return the normalised title and tags, or raise ValidationError listing every problem."""
    problems: dict[str, str] = {}
    clean_title = title.strip()
    if not 1 <= len(clean_title) <= 200:
        problems["title"] = "must be 1-200 characters"
    clean_tags = tuple(tags)
    if len(clean_tags) > 10 or len(set(clean_tags)) != len(clean_tags):
        problems["tags"] = "at most 10 unique tags"
    elif any(not 1 <= len(t) <= 30 or t != t.lower() for t in clean_tags):
        problems["tags"] = "each tag must be 1-30 lowercase characters"
    if problems:
        raise ValidationError(problems)
    return clean_title, clean_tags


class TaskRepository(Protocol):
    def add(self, title: str, tags: Iterable[str] = ()) -> Task: ...
    def get(self, task_id: int) -> Task | None: ...
    def update(self, task: Task) -> None: ...
    def all(self) -> Iterator[Task]: ...


class InMemoryTaskRepository:
    def __init__(self) -> None:
        self._tasks: dict[int, Task] = {}
        self._next_id = 1

    def add(self, title: str, tags: Iterable[str] = ()) -> Task:
        clean_title, clean_tags = validate(title, tags)
        task = Task(
            self._next_id,
            clean_title,
            tags=clean_tags,
            created_at=datetime(2030, 1, 1, 9, 0, tzinfo=UTC),
        )
        self._tasks[task.id] = task
        self._next_id += 1
        return task

    def get(self, task_id: int) -> Task | None:
        return self._tasks.get(task_id)

    def update(self, task: Task) -> None:
        self._tasks[task.id] = task

    def all(self) -> Iterator[Task]:
        return iter(self._tasks.values())


def by_status(tasks: Iterable[Task], status: Status) -> Iterator[Task]:
    return (t for t in tasks if t.status is status)


def tag_counts(tasks: Iterable[Task]) -> dict[str, int]:
    counts: dict[str, int] = {}
    for task in tasks:
        for tag in task.tags:
            counts[tag] = counts.get(tag, 0) + 1
    return dict(sorted(counts.items()))


def main() -> None:
    repo: TaskRepository = InMemoryTaskRepository()
    repo.add("  Write docs  ", ["docs", "p1"])
    repo.add("Fix bug", ["bug", "p1"])
    repo.add("Ship it")

    if (task := repo.get(2)) is not None:
        repo.update(replace(task, status=Status.IN_PROGRESS, due_date=date(2030, 1, 31)))

    for task in repo.all():
        due = task.due_date.isoformat() if task.due_date else "-"
        print(
            f"{task.id} {task.title!r:14} {task.status.value:11} due={due} tags={list(task.tags)}"
        )

    print("in progress:", [t.title for t in by_status(repo.all(), Status.IN_PROGRESS)])
    print("tag counts:", tag_counts(repo.all()))
    print("missing:", repo.get(99))

    try:
        repo.add("   ", ["Bad Tag"])
    except ValidationError as err:
        print("rejected:", err.fields)


if __name__ == "__main__":
    main()
