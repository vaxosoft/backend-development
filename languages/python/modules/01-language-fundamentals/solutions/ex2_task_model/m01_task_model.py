"""Exercise 2: model a Task with a dataclass, an Enum and collected validation errors."""

from collections.abc import Mapping
from dataclasses import dataclass
from datetime import date
from enum import StrEnum
from typing import Any


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

    def to_dict(self) -> dict[str, Any]:
        """Wire format: status as its value, due_date as YYYY-MM-DD or None, tags as a list."""
        return {
            "id": self.id,
            "title": self.title,
            "status": self.status.value,
            "due_date": self.due_date.isoformat() if self.due_date else None,
            "tags": list(self.tags),
            "owner_id": self.owner_id,
        }


def parse_task(task_id: int, data: Mapping[str, Any]) -> Task:
    """Build a Task from untrusted input, or raise ValidationError with one message per bad field.

    Rules (keys of `ValidationError.fields` are the input field names):
    - title: required str, stripped, 1-200 characters
    - status: optional, one of the Status values, default "todo"
    - due_date: optional "YYYY-MM-DD" string or None
    - tags: optional list of 0-10 unique strings, each 1-30 characters and lowercase
    - owner_id: optional int (not bool) or None
    Every invalid field must be reported in one ValidationError, not only the first.
    """
    problems: dict[str, str] = {}

    title = data.get("title")
    clean_title = title.strip() if isinstance(title, str) else ""
    if not 1 <= len(clean_title) <= 200:
        problems["title"] = "must be 1-200 characters"

    status = Status.TODO
    try:
        status = Status(data.get("status", Status.TODO))
    except ValueError:
        problems["status"] = "must be one of todo, in_progress, done"

    due: date | None = None
    raw_due = data.get("due_date")
    if raw_due is not None:
        try:
            if not isinstance(raw_due, str):
                raise TypeError
            due = date.fromisoformat(raw_due)
        except TypeError, ValueError:
            problems["due_date"] = "must be a YYYY-MM-DD date"

    tags: tuple[str, ...] = ()
    raw_tags = data.get("tags", [])
    if (
        not isinstance(raw_tags, list)
        or not all(isinstance(t, str) for t in raw_tags)
        or len(raw_tags) > 10
        or len(set(raw_tags)) != len(raw_tags)
        or any(not 1 <= len(t) <= 30 or t != t.lower() for t in raw_tags)
    ):
        problems["tags"] = "must be at most 10 unique lowercase strings of 1-30 characters"
    else:
        tags = tuple(raw_tags)

    owner_id = data.get("owner_id")
    if owner_id is not None and (isinstance(owner_id, bool) or not isinstance(owner_id, int)):
        problems["owner_id"] = "must be an integer"

    if problems:
        raise ValidationError(problems)
    return Task(task_id, clean_title, status, due, tags, owner_id)
