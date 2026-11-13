import uuid
from dataclasses import dataclass, field
from datetime import UTC, datetime
from enum import StrEnum
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict

type JobType = Literal["reverse", "sleep", "fail"]
type JobResult = str | dict[str, int] | None


class JobStatus(StrEnum):
    QUEUED = "queued"
    RUNNING = "running"
    SUCCEEDED = "succeeded"
    FAILED = "failed"
    CANCELLED = "cancelled"

    @property
    def is_final(self) -> bool:
        return self in {JobStatus.SUCCEEDED, JobStatus.FAILED, JobStatus.CANCELLED}


@dataclass(slots=True)
class Job:
    """Mutable job record. Only the pool changes it, always from the event loop thread."""

    type: JobType
    payload: dict[str, Any]
    id: str = field(default_factory=lambda: str(uuid.uuid7()))
    status: JobStatus = JobStatus.QUEUED
    attempts: int = 0
    result: JobResult = None
    error: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(UTC))


class JobRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    type: JobType
    payload: dict[str, Any] = {}


class JobAccepted(BaseModel):
    id: str


class JobView(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: str
    type: JobType
    status: JobStatus
    attempts: int
    result: JobResult
    error: str | None
    created_at: datetime


class JobList(BaseModel):
    items: list[JobView]
    total: int
    limit: int
    offset: int
