"""Task Tracker wire format: decode a create request, build the response and the 422 envelope."""

import json
from datetime import UTC, date, datetime
from enum import StrEnum

from pydantic import AwareDatetime, BaseModel, ConfigDict, ValidationError, field_validator


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class TaskCreate(BaseModel):
    """What clients may send. Unknown fields are errors; read-only fields are not accepted."""

    model_config = ConfigDict(extra="forbid")

    title: str
    status: Status = Status.TODO
    due_date: date | None = None
    tags: list[str] = []
    owner_id: int | None = None

    @field_validator("title")
    @classmethod
    def check_title(cls, value: str) -> str:
        value = value.strip()
        if not 1 <= len(value) <= 200:
            raise ValueError("must be 1-200 characters")
        return value

    @field_validator("tags")
    @classmethod
    def check_tags(cls, value: list[str]) -> list[str]:
        if len(value) > 10:
            raise ValueError("must have at most 10 tags")
        if len(set(value)) != len(value):
            raise ValueError("must not contain duplicates")
        if any(not 1 <= len(t) <= 30 or t != t.lower() for t in value):
            raise ValueError("each tag must be 1-30 lowercase characters")
        return value


class Task(BaseModel):
    """What the server returns. The read-only fields are part of the output, never of the input."""

    id: int
    title: str
    status: Status
    due_date: date | None
    tags: list[str]
    owner_id: int | None
    created_at: AwareDatetime


def envelope(err: ValidationError) -> dict[str, object]:
    fields = {
        ".".join(str(p) for p in e["loc"]): e["msg"].removeprefix("Value error, ")
        for e in err.errors()
    }
    return {"error": {"code": "validation_failed", "message": "invalid task", "fields": fields}}


def create_task(body: bytes, next_id: int) -> tuple[int, dict[str, object]]:
    try:
        data = TaskCreate.model_validate_json(body)
    except ValidationError as err:
        # Malformed JSON is also a ValidationError in pydantic; tell it apart by its error type.
        if err.errors()[0]["type"] == "json_invalid":
            return 400, {"error": {"code": "bad_request", "message": "malformed JSON"}}
        return 422, envelope(err)
    created = datetime(2030, 1, 1, 9, 0, tzinfo=UTC)
    task = Task(id=next_id, created_at=created, **data.model_dump())
    return 201, task.model_dump(mode="json")


requests = [
    b'{"title": "Write docs", "tags": ["docs", "p1"], "due_date": "2030-01-31"}',
    b'{"title": "", "status": "later", "tags": ["Docs", "docs"], "id": 9}',
    b'{"title": "oops",}',
]
for body in requests:
    status, payload = create_task(body, next_id=1)
    print(status, json.dumps(payload))

# Lists share one envelope everywhere.
tasks = [Task.model_validate(create_task(b'{"title": "Item %d"}' % n, n)[1]) for n in (1, 2)]
page = {"items": [t.model_dump(mode="json") for t in tasks], "total": 2, "limit": 20, "offset": 0}
print(json.dumps(page))
