"""Custom (de)serialization with pydantic: validators, serializers and aliases."""

from datetime import UTC, datetime
from decimal import Decimal
from typing import Annotated

from pydantic import (
    AwareDatetime,
    BaseModel,
    BeforeValidator,
    ConfigDict,
    PlainSerializer,
    StringConstraints,
    ValidationError,
    field_serializer,
    field_validator,
)
from pydantic.alias_generators import to_camel

# A reusable annotated type: normalise first, then constrain.
Tag = Annotated[
    str,
    BeforeValidator(lambda v: v.strip().lower() if isinstance(v, str) else v),
    StringConstraints(min_length=1, max_length=30),
]
# Decimals travel as strings so no precision is lost on the way through JSON numbers.
Hours = Annotated[Decimal, PlainSerializer(lambda d: str(d), return_type=str, when_used="json")]


class TaskDto(BaseModel):
    model_config = ConfigDict(alias_generator=to_camel, populate_by_name=True, extra="forbid")

    title: str
    tags: list[Tag] = []
    estimate_hours: Hours | None = None
    created_at: AwareDatetime

    @field_validator("title")
    @classmethod
    def trim_title(cls, value: str) -> str:
        value = value.strip()
        if not 1 <= len(value) <= 200:
            raise ValueError("must be 1-200 characters")
        return value

    @field_validator("tags")
    @classmethod
    def unique_tags(cls, value: list[str]) -> list[str]:
        if len(set(value)) != len(value):
            raise ValueError("must not contain duplicates")
        return value

    @field_serializer("created_at")
    def format_created_at(self, value: datetime) -> str:
        return value.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ")


incoming = (
    '{"title": "  Ship it  ", "tags": [" Docs", "P1 "], "estimateHours": "1.50",'
    ' "createdAt": "2030-01-01T10:30:15.123456+01:00"}'
)
dto = TaskDto.model_validate_json(incoming)
print(repr(dto.title), dto.tags, repr(dto.estimate_hours))

# by_alias controls the outgoing names; the Python attributes stay snake_case.
print(dto.model_dump_json(by_alias=True))
print(dto.model_dump_json())

try:
    TaskDto.model_validate({"title": " ", "tags": ["a", "A"], "created_at": "2030-01-01T00:00:00Z"})
except ValidationError as err:
    for e in err.errors():
        print(e["loc"], "-", e["msg"])
