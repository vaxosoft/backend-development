from datetime import date, datetime
from enum import StrEnum
from typing import Literal

from pydantic import BaseModel, Field, field_serializer

type Sort = Literal["created_at", "-created_at", "due_date", "-due_date"]

MAX_LIMIT = 100


class TaskStatus(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class TaskIn(BaseModel):
    """Body of POST /tasks and PUT /tasks/{id}."""

    # TODO: enforce the rules of the spec (README, "Task rules"). Pydantic gives you
    # StringConstraints / Field(max_length=) for most of them; due_date and the tag
    # rules need a validator.
    title: str
    status: TaskStatus = TaskStatus.TODO
    due_date: date | None = None
    tags: list[str] = Field(default_factory=list)
    owner_id: int | None = None


class Task(BaseModel):
    """What the API returns. A separate model from TaskIn: id and created_at are read-only."""

    id: int
    title: str
    status: TaskStatus
    due_date: date | None
    tags: list[str]
    owner_id: int | None
    created_at: datetime

    @field_serializer("created_at")
    def _rfc3339(self, value: datetime) -> str:
        return value.strftime("%Y-%m-%dT%H:%M:%SZ")


class TaskList(BaseModel):
    items: list[Task]
    total: int
    limit: int
    offset: int
