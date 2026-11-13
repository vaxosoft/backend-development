"""Task Tracker over HTTP: routes, dependency injection, request-id middleware, 404 handling.

Run:  python 05_task_server.py   (ADDR=host:port to change the address)
"""

import uuid
from collections.abc import Awaitable, Callable
from typing import Annotated

from fastapi import Depends, FastAPI, HTTPException, Request, Response
from m02_helpers import serve
from pydantic import BaseModel, Field


class TaskIn(BaseModel):
    title: Annotated[str, Field(min_length=1, max_length=200)]
    tags: list[str] = []


class Task(BaseModel):
    id: int
    title: str
    status: str = "todo"
    tags: list[str] = []


class TaskStore:
    def __init__(self) -> None:
        self._tasks: dict[int, Task] = {}

    def add(self, data: TaskIn) -> Task:
        task = Task(id=len(self._tasks) + 1, **data.model_dump())
        self._tasks[task.id] = task
        return task

    def get(self, task_id: int) -> Task | None:
        return self._tasks.get(task_id)

    def all(self) -> list[Task]:
        return list(self._tasks.values())


def create_app(store: TaskStore | None = None) -> FastAPI:
    app = FastAPI(title="Task Tracker (module 02)")
    shared_store = store or TaskStore()

    def get_store() -> TaskStore:
        return shared_store

    @app.middleware("http")
    async def request_id(
        request: Request, call_next: Callable[[Request], Awaitable[Response]]
    ) -> Response:
        rid = request.headers.get("X-Request-ID") or uuid.uuid4().hex
        response = await call_next(request)
        response.headers["X-Request-ID"] = rid
        return response

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/tasks")
    def list_tasks(store: Annotated[TaskStore, Depends(get_store)]) -> list[Task]:
        return store.all()

    @app.post("/tasks", status_code=201)
    def create_task(
        data: TaskIn, response: Response, store: Annotated[TaskStore, Depends(get_store)]
    ) -> Task:
        task = store.add(data)
        response.headers["Location"] = f"/tasks/{task.id}"
        return task

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int, store: Annotated[TaskStore, Depends(get_store)]) -> Task:
        task = store.get(task_id)
        if task is None:
            raise HTTPException(status_code=404, detail=f"task {task_id} not found")
        return task

    return app


if __name__ == "__main__":
    serve(create_app())
