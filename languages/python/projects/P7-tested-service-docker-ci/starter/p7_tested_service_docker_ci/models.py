import re
from datetime import date, datetime
from enum import StrEnum
from typing import Annotated

from pydantic import BaseModel, ConfigDict, Field, StringConstraints, field_validator

Title = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)]
Tag = Annotated[str, StringConstraints(min_length=1, max_length=30)]


class TaskStatus(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class TaskCreate(BaseModel):
    model_config = ConfigDict(extra="forbid")

    title: Title
    status: TaskStatus = TaskStatus.TODO
    due_date: date | None = None
    tags: list[Tag] = Field(default_factory=list, max_length=10)

    @field_validator("tags")
    @classmethod
    def tags_are_lowercase_and_unique(cls, tags: list[str]) -> list[str]:
        if any(not re.fullmatch(r"[a-z0-9_-]+", tag) for tag in tags):
            raise ValueError("tags must be lowercase letters, digits, '-' or '_'")
        if len(set(tags)) != len(tags):
            raise ValueError("tags must be unique")
        return tags


class Task(BaseModel):
    id: int
    title: str
    status: TaskStatus
    due_date: date | None
    tags: list[str]
    created_at: datetime
