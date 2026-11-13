"""pydantic: decode, validate and encode in one declaration."""

from datetime import date, datetime
from enum import StrEnum

from pydantic import AwareDatetime, BaseModel, ConfigDict, ValidationError


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class Task(BaseModel):
    model_config = ConfigDict(extra="forbid")

    id: int
    title: str
    status: Status = Status.TODO
    due_date: date | None = None
    tags: list[str] = []
    created_at: AwareDatetime


raw = (
    '{"id": 1, "title": "Write docs", "due_date": "2030-01-31",'
    ' "created_at": "2030-01-01T09:00:00Z"}'
)
task = Task.model_validate_json(raw)
print(repr(task))

# mode="json" turns date/datetime/enum into JSON-safe values; the default mode keeps Python objects.
print(task.model_dump())
print(task.model_dump(mode="json"))
print(task.model_dump_json())

# Lax mode coerces compatible values: the string "7" becomes 7. Strict mode would refuse it.
print(Task.model_validate({"id": "7", "title": "x", "created_at": "2030-01-01T00:00:00Z"}).id)


def show(data: dict[str, object]) -> None:
    try:
        Task.model_validate(data)
    except ValidationError as err:
        for e in err.errors():
            print(e["loc"], e["type"], "-", e["msg"])


base: dict[str, object] = {"id": 1, "title": "x", "created_at": "2030-01-01T00:00:00Z"}
show({**base, "status": "blocked"})  # not in the enum
show({**base, "colour": "red"})  # extra="forbid": unknown fields are errors, not ignored
show({**base, "created_at": "2030-01-01T00:00:00"})  # naive datetimes are rejected
show({"title": "x"})  # missing fields are reported together


# Missing, null and present are three different states in a partial update.
class TaskUpdate(BaseModel):
    title: str | None = None
    due_date: date | None = None


for body in ('{"title": "New"}', '{"due_date": null}'):
    update = TaskUpdate.model_validate_json(body)
    print(body, "->", sorted(update.model_fields_set), update.model_dump(exclude_unset=True))

# datetime values keep their offset; convert explicitly when you need UTC.
parsed = datetime.fromisoformat("2030-01-01T10:00:00+01:00")
print(parsed.isoformat(), "->", parsed.astimezone(task.created_at.tzinfo).isoformat())
