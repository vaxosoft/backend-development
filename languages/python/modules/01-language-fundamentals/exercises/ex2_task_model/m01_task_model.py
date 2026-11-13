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
        raise NotImplementedError


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
    raise NotImplementedError
