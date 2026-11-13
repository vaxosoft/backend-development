from datetime import date
from enum import StrEnum
from typing import Any

from pydantic import BaseModel, ConfigDict, ValidationError, field_validator


class PatchError(Exception):
    def __init__(self, fields: dict[str, str]) -> None:
        super().__init__(f"invalid patch: {fields}")
        self.fields = fields


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class TaskPatch(BaseModel):
    # title, status and tags are not nullable, so an explicit null fails validation.
    # The defaults are never used: only fields in `model_fields_set` are applied.
    model_config = ConfigDict(extra="forbid")

    title: str = ""
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
        if len(set(value)) != len(value):
            raise ValueError("must not contain duplicates")
        return value


def apply_patch(task: dict[str, Any], body: str) -> dict[str, Any]:
    try:
        patch = TaskPatch.model_validate_json(body)
    except ValidationError as err:
        fields: dict[str, str] = {}
        for e in err.errors():
            if not e["loc"]:
                message = (
                    "malformed JSON" if e["type"] == "json_invalid" else "must be a JSON object"
                )
                fields["body"] = message
            else:
                fields[".".join(str(p) for p in e["loc"])] = e["msg"].removeprefix("Value error, ")
        raise PatchError(fields) from err
    changes = patch.model_dump(mode="json", include=patch.model_fields_set)
    return {**task, **changes}
