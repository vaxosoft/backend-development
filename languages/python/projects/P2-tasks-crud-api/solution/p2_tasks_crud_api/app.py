from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass
from typing import Annotated, Any

from fastapi import FastAPI, Query, Response
from starlette.exceptions import HTTPException

from .errors import register_error_handlers
from .models import MAX_LIMIT, Sort, Task, TaskIn, TaskList, TaskStatus
from .repository import Clock, SqliteTaskRepository, utc_now

PATCHABLE = ("title", "status", "due_date", "tags")


@dataclass(slots=True, frozen=True)
class Config:
    db_path: str = ":memory:"
    clock: Clock = utc_now


def _not_found() -> HTTPException:
    return HTTPException(status_code=404, detail="task not found")


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
        task = repo.create(data)
        response.headers["Location"] = f"/tasks/{task.id}"
        return task

    @app.get("/tasks")
    def list_tasks(
        status: TaskStatus | None = None,
        tag: str | None = None,
        limit: Annotated[int, Query(ge=1)] = 20,
        offset: Annotated[int, Query(ge=0)] = 0,
        sort: Sort = "-created_at",
    ) -> TaskList:
        limit = min(limit, MAX_LIMIT)
        items, total = repo.list(status=status, tag=tag, sort=sort, limit=limit, offset=offset)
        return TaskList(items=items, total=total, limit=limit, offset=offset)

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int) -> Task:
        task = repo.get(task_id)
        if task is None:
            raise _not_found()
        return task

    @app.put("/tasks/{task_id}")
    def replace_task(task_id: int, data: TaskIn) -> Task:
        task = repo.replace(task_id, data)
        if task is None:
            raise _not_found()
        return task

    @app.patch("/tasks/{task_id}")
    def patch_task(task_id: int, body: dict[str, Any]) -> Task:
        current = repo.get(task_id)
        if current is None:
            raise _not_found()
        merged = current.model_dump(mode="json", include={*PATCHABLE, "owner_id"})
        merged.update({key: value for key, value in body.items() if key in PATCHABLE})
        task = repo.replace(task_id, TaskIn.model_validate(merged))
        if task is None:
            raise _not_found()
        return task

    @app.delete("/tasks/{task_id}", status_code=204)
    def delete_task(task_id: int) -> None:
        if not repo.delete(task_id):
            raise _not_found()

    return app
