from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import Annotated, Any

from fastapi import FastAPI, Query, Response

from .errors import register_error_handlers
from .models import Sort, Task, TaskIn, TaskList, TaskStatus
from .repository import Clock, SqliteTaskRepository, utc_now

PATCHABLE = ("title", "status", "due_date", "tags")


@dataclass(slots=True, frozen=True)
class Config:
    db_path: str = ":memory:"
    clock: Clock = utc_now


def create_app(config: Config | None = None) -> FastAPI:
    config = config or Config()
    repo = SqliteTaskRepository(config.db_path, config.clock)

    @asynccontextmanager
    async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
        yield
        repo.close()

    app = FastAPI(title="P2 Tasks CRUD API", lifespan=lifespan)
    register_error_handlers(app)

    @app.post("/tasks", status_code=201)
    def create_task(data: TaskIn, response: Response) -> Task:
        # TODO: store the task and set the Location header to /tasks/{id}.
        raise NotImplementedError

    @app.get("/tasks")
    def list_tasks(
        status: TaskStatus | None = None,
        tag: str | None = None,
        limit: Annotated[int, Query(ge=1)] = 20,
        offset: Annotated[int, Query(ge=0)] = 0,
        sort: Sort = "-created_at",
    ) -> TaskList:
        # TODO: clamp limit to MAX_LIMIT (models.py), ask the repository, return the list envelope.
        raise NotImplementedError

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int) -> Task:
        # TODO: 404 (HTTPException) when the id is unknown.
        raise NotImplementedError

    @app.put("/tasks/{task_id}")
    def replace_task(task_id: int, data: TaskIn) -> Task:
        # TODO: full replace; 404 when unknown.
        raise NotImplementedError

    @app.patch("/tasks/{task_id}")
    def patch_task(task_id: int, body: dict[str, Any]) -> Task:
        # TODO: only PATCHABLE keys count and absent keys stay unchanged (null due_date clears).
        # Merge them over the stored task and validate the result with TaskIn.model_validate.
        raise NotImplementedError

    @app.delete("/tasks/{task_id}", status_code=204)
    def delete_task(task_id: int) -> None:
        # TODO: 204 without a body; 404 when unknown.
        raise NotImplementedError

    return app
