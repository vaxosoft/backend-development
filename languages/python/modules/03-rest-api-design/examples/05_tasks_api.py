"""A complete Task Tracker REST API: CRUD, PATCH semantics, listing, envelope, OpenAPI.

Run:  FIXED_NOW=2030-01-01T09:00:00Z python 05_tasks_api.py   (ADDR=host:port to change the address)
Docs: /docs (Swagger UI) and /openapi.json
"""

from typing import Annotated, Any

from fastapi import APIRouter, FastAPI, Query, Response
from m03_errors import ApiError, install_error_handlers, serve
from m03_tasks import (
    DEFAULT_LIMIT,
    MAX_LIMIT,
    Clock,
    SortKey,
    Status,
    Task,
    TaskCreate,
    TaskPatch,
    TaskStore,
    clock_from_env,
)
from pydantic import BaseModel


class ErrorBody(BaseModel):
    code: str
    message: str
    fields: dict[str, str] | None = None


class ErrorEnvelope(BaseModel):
    error: ErrorBody


class TaskPage(BaseModel):
    items: list[Task]
    total: int
    limit: int
    offset: int


ERRORS: dict[int | str, dict[str, Any]] = {
    400: {"model": ErrorEnvelope, "description": "Malformed JSON or bad query value"},
    404: {"model": ErrorEnvelope, "description": "Unknown task"},
    422: {"model": ErrorEnvelope, "description": "Well-formed but invalid data"},
}


def create_app(clock: Clock | None = None) -> FastAPI:
    app = FastAPI(
        title="Task Tracker API",
        version="1.0.0",
        description="Reference REST API for the Python track.",
    )
    install_error_handlers(app)
    store = TaskStore(clock or clock_from_env())
    api = APIRouter(prefix="/v1/tasks", tags=["tasks"], responses=ERRORS)

    def found(task: Task | None, task_id: int) -> Task:
        if task is None:
            raise ApiError(404, f"task {task_id} not found")
        return task

    @api.post("", status_code=201, summary="Create a task")
    def create(data: TaskCreate, response: Response) -> Task:
        task = store.create(data)
        response.headers["Location"] = f"/v1/tasks/{task.id}"
        return task

    @api.get("", summary="List tasks")
    def list_tasks(
        status: Status | None = None,
        tag: str | None = None,
        sort: SortKey = "-created_at",
        limit: Annotated[int, Query(ge=1, description="Clamped to 100")] = DEFAULT_LIMIT,
        offset: Annotated[int, Query(ge=0)] = 0,
    ) -> TaskPage:
        limit = min(limit, MAX_LIMIT)
        items, total = store.query(status=status, tag=tag, sort=sort, limit=limit, offset=offset)
        return TaskPage(items=items, total=total, limit=limit, offset=offset)

    @api.get("/{task_id}", summary="Get one task")
    def read(task_id: int) -> Task:
        return found(store.get(task_id), task_id)

    @api.put("/{task_id}", summary="Replace a task")
    def replace(task_id: int, data: TaskCreate) -> Task:
        return found(store.replace(task_id, data), task_id)

    @api.patch("/{task_id}", summary="Change some fields of a task")
    def patch(task_id: int, changes: TaskPatch) -> Task:
        return found(store.patch(task_id, changes), task_id)

    @api.delete("/{task_id}", status_code=204, summary="Delete a task")
    def delete(task_id: int) -> None:
        found(store.get(task_id), task_id)
        store.delete(task_id)

    @app.get("/healthz", tags=["ops"])
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    app.include_router(api)
    return app


if __name__ == "__main__":
    serve(create_app())
