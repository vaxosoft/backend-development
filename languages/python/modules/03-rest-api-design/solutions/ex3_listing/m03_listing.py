"""Exercise 3: filtering, sorting, pagination and an idempotent POST."""

import threading
from collections.abc import Callable
from datetime import date, datetime
from enum import StrEnum
from typing import Annotated, Any, Literal

from fastapi import FastAPI, Header, HTTPException, Query, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field
from starlette.exceptions import HTTPException as StarletteHTTPException

DEFAULT_LIMIT = 20
MAX_LIMIT = 100
Clock = Callable[[], datetime]


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


class NewTask(BaseModel):
    title: str = Field(min_length=1, max_length=200)
    status: Status = Status.TODO
    due_date: date | None = None
    tags: list[str] = Field(default_factory=list)


class Task(BaseModel):
    id: int
    title: str
    status: Status
    due_date: date | None
    tags: list[str]
    created_at: datetime


class Page(BaseModel):
    items: list[Task]
    total: int
    limit: int
    offset: int


class TaskStore:
    def __init__(self, clock: Clock) -> None:
        self._clock = clock
        self._tasks: list[Task] = []

    def add(self, data: NewTask) -> Task:
        task = Task(id=len(self._tasks) + 1, created_at=self._clock(), **data.model_dump())
        self._tasks.append(task)
        return task

    def all(self) -> list[Task]:
        return list(self._tasks)


def envelope(status: int, code: str, message: str, fields: dict[str, str] | None = None) -> Any:
    body: dict[str, Any] = {"code": code, "message": message}
    if fields:
        body["fields"] = fields
    return JSONResponse({"error": body}, status_code=status)


def install_error_handlers(app: FastAPI) -> None:
    """Given: query/path errors -> 400 bad_request, body errors -> 422, HTTP errors keep status."""

    @app.exception_handler(RequestValidationError)
    async def on_validation(request: Request, exc: RequestValidationError) -> Any:
        errors = exc.errors()
        fields = {".".join(str(p) for p in e["loc"][1:]) or "body": e["msg"] for e in errors}
        if any(e["type"] == "json_invalid" for e in errors):
            return envelope(400, "bad_request", "malformed JSON body")
        if any(e["loc"][0] != "body" for e in errors):
            return envelope(400, "bad_request", "invalid request parameters", fields)
        return envelope(422, "validation_failed", "invalid request", fields)

    @app.exception_handler(StarletteHTTPException)
    async def on_http(request: Request, exc: StarletteHTTPException) -> Any:
        codes = {404: "not_found", 405: "method_not_allowed", 409: "conflict"}
        return envelope(exc.status_code, codes.get(exc.status_code, "error"), str(exc.detail))


def create_app(clock: Clock) -> FastAPI:
    """Add these routes (the store, models and error handlers above are given):

    GET /tasks?status=&tag=&q=&sort=&limit=&offset=
        - status: exact match on one of the Status values; tag: the task has this tag;
          q: case-insensitive substring of the title. Filters combine with AND.
        - sort: one of created_at, -created_at (default), due_date, -due_date. Tasks without a
          due date come LAST for both due_date directions; ties are broken by ascending id.
        - limit: default 20, at least 1, values above 100 are CLAMPED to 100 (not rejected);
          offset: default 0, at least 0. Bad values (including an unknown sort/status) -> 400.
        - 200 body: {"items": [...], "total": <matches before paging>, "limit": <effective>,
          "offset": <offset>}

    POST /tasks   body NewTask -> 201 + Location: /tasks/<id> + the Task.
        - With header "Idempotency-Key": the first request creates the task. Repeating the same
          key with an identical body returns the SAME task (201, same id), adds no new task and
          sets the header "Idempotency-Replayed: true". The same key with a different body -> 409
          {"error": {"code": "conflict", ...}}. Requests without the header always create.
    """
    app = FastAPI()
    install_error_handlers(app)
    store = TaskStore(clock)
    replays: dict[str, tuple[NewTask, Task]] = {}
    lock = threading.Lock()

    @app.post("/tasks", status_code=201)
    def create(
        data: NewTask,
        response: Response,
        idempotency_key: Annotated[str | None, Header()] = None,
    ) -> Task:
        with lock:
            if idempotency_key is None:
                task = store.add(data)
            elif (previous := replays.get(idempotency_key)) is None:
                task = store.add(data)
                replays[idempotency_key] = (data, task)
            elif previous[0] != data:
                raise HTTPException(409, "Idempotency-Key was used with a different request")
            else:
                task = previous[1]
                response.headers["Idempotency-Replayed"] = "true"
        response.headers["Location"] = f"/tasks/{task.id}"
        return task

    @app.get("/tasks")
    def list_tasks(
        status: Status | None = None,
        tag: str | None = None,
        q: str | None = None,
        sort: Literal["created_at", "-created_at", "due_date", "-due_date"] = "-created_at",
        limit: Annotated[int, Query(ge=1)] = DEFAULT_LIMIT,
        offset: Annotated[int, Query(ge=0)] = 0,
    ) -> Page:
        limit = min(limit, MAX_LIMIT)
        matches = [
            t
            for t in store.all()
            if (status is None or t.status is status)
            and (tag is None or tag in t.tags)
            and (q is None or q.lower() in t.title.lower())
        ]
        sign = -1 if sort.startswith("-") else 1
        if sort.endswith("created_at"):
            matches.sort(key=lambda t: (sign * t.created_at.timestamp(), t.id))
        else:
            matches.sort(
                key=lambda t: (
                    t.due_date is None,
                    sign * t.due_date.toordinal() if t.due_date else 0,
                    t.id,
                )
            )
        return Page(
            items=matches[offset : offset + limit], total=len(matches), limit=limit, offset=offset
        )

    return app
