import re
from datetime import date, datetime
from enum import StrEnum
from typing import Annotated, Any, Literal

from pydantic import (
    AfterValidator,
    BaseModel,
    Field,
    StrictInt,
    StringConstraints,
    field_serializer,
    field_validator,
)

type Sort = Literal["created_at", "-created_at", "due_date", "-due_date"]

MAX_LIMIT = 100
_DATE = re.compile(r"\d{4}-\d{2}-\d{2}")


class TaskStatus(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


def _lowercase(tag: str) -> str:
    if tag != tag.lower():
        raise ValueError("must be lowercase")
    return tag


Title = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)]
Tag = Annotated[str, StringConstraints(min_length=1, max_length=30), AfterValidator(_lowercase)]


class TaskIn(BaseModel):
    """Body of POST /tasks and PUT /tasks/{id}."""

    title: Title
    status: TaskStatus = TaskStatus.TODO
    due_date: date | None = None
    tags: Annotated[list[Tag], Field(max_length=10)] = Field(default_factory=list)
    owner_id: StrictInt | None = None

    @field_validator("due_date", mode="before")
    @classmethod
    def _iso_date_only(cls, value: Any) -> Any:
        # date.fromisoformat() also accepts "20300131"; the wire format is strictly YYYY-MM-DD.
        if value is not None and not (isinstance(value, str) and _DATE.fullmatch(value)):
            raise ValueError("must be a date as YYYY-MM-DD")
        return value

    @field_validator("tags")
    @classmethod
    def _unique_tags(cls, tags: list[str]) -> list[str]:
        if len(set(tags)) != len(tags):
            raise ValueError("must not contain duplicates")
        return tags


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
