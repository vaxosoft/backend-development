"""Exercise 3: a FastAPI app whose collaborators can be replaced in tests.

Implement `create_app()`. Everything else in this file is given. Requirements:

* Install the error handlers with `install_error_handlers(app)`.
* Routes must get the store and the clock ONLY through `Depends(get_store)` and
  `Depends(get_clock)`; never through module-level globals, so tests can override them.
* `POST /tasks` (body: `TaskIn`) -> 201 with `Location: /tasks/<id>` and `task_to_json(task)`;
  the store's `add(title, created_at)` is called with the clock's current time.
* `GET /tasks/{task_id}` -> 200 `task_to_json(task)`, or `ApiError(404, "not_found", ...)`.
* `GET /tasks` -> 200 `{"items": [task_to_json(...)...], "total": N}`.
"""

from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime
from functools import lru_cache
from typing import Annotated, Protocol

from fastapi import Depends, FastAPI, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, ConfigDict, Field


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    created_at: datetime


class TaskStore(Protocol):
    def add(self, title: str, created_at: datetime) -> Task: ...
    def get(self, task_id: int) -> Task | None: ...
    def list_all(self) -> list[Task]: ...


class InMemoryStore:
    def __init__(self) -> None:
        self._tasks: list[Task] = []

    def add(self, title: str, created_at: datetime) -> Task:
        task = Task(len(self._tasks) + 1, title, "todo", created_at)
        self._tasks.append(task)
        return task

    def get(self, task_id: int) -> Task | None:
        return next((t for t in self._tasks if t.id == task_id), None)

    def list_all(self) -> list[Task]:
        return list(self._tasks)


type Clock = Callable[[], datetime]


@lru_cache
def get_store() -> TaskStore:
    return InMemoryStore()


def get_clock() -> Clock:
    return lambda: datetime.now(UTC)


class TaskIn(BaseModel):
    model_config = ConfigDict(str_strip_whitespace=True)
    title: str = Field(min_length=1, max_length=200)


class ApiError(Exception):
    def __init__(self, status: int, code: str, message: str) -> None:
        super().__init__(message)
        self.status, self.code, self.message = status, code, message


def task_to_json(task: Task) -> dict[str, object]:
    return {
        "id": task.id,
        "title": task.title,
        "status": task.status,
        "created_at": task.created_at.astimezone(UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
    }


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(ApiError)
    def api_error(_: Request, exc: ApiError) -> JSONResponse:
        body = {"error": {"code": exc.code, "message": exc.message}}
        return JSONResponse(body, status_code=exc.status)

    @app.exception_handler(RequestValidationError)
    def invalid(_: Request, __: RequestValidationError) -> JSONResponse:
        body = {"error": {"code": "validation_failed", "message": "invalid task"}}
        return JSONResponse(body, status_code=422)


Store = Annotated[TaskStore, Depends(get_store)]
Now = Annotated[Clock, Depends(get_clock)]


def create_app() -> FastAPI:
    app = FastAPI()
    install_error_handlers(app)

    @app.post("/tasks", status_code=201)
    def create_task(body: TaskIn, store: Store, now: Now, response: Response) -> dict[str, object]:
        task = store.add(body.title, now())
        response.headers["Location"] = f"/tasks/{task.id}"
        return task_to_json(task)

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int, store: Store) -> dict[str, object]:
        task = store.get(task_id)
        if task is None:
            raise ApiError(404, "not_found", f"task {task_id} not found")
        return task_to_json(task)

    @app.get("/tasks")
    def list_tasks(store: Store) -> dict[str, object]:
        tasks = store.list_all()
        return {"items": [task_to_json(t) for t in tasks], "total": len(tasks)}

    return app
