"""API versioning in the URL and idempotency: PUT/DELETE semantics and Idempotency-Key for POST.

Run:  FIXED_NOW=2030-01-01T09:00:00Z python 04_versioning_and_idempotency.py   (ADDR=host:port)
"""

import threading
from typing import Annotated

from fastapi import APIRouter, FastAPI, Header, Response
from m03_errors import ApiError, install_error_handlers, serve
from m03_tasks import Task, TaskCreate, TaskStore, clock_from_env


def create_app() -> FastAPI:
    app = FastAPI(title="Idempotency demo")
    install_error_handlers(app)
    store = TaskStore(clock_from_env())
    replays: dict[str, tuple[str, Task]] = {}  # Idempotency-Key -> (request fingerprint, response)
    lock = threading.Lock()  # `def` handlers run in threads: check-then-create must be atomic

    v1 = APIRouter(prefix="/v1", tags=["tasks"])

    @v1.post("/tasks", status_code=201)
    def create(
        data: TaskCreate,
        response: Response,
        idempotency_key: Annotated[str | None, Header(min_length=1, max_length=100)] = None,
    ) -> Task:
        with lock:
            if idempotency_key is None:
                task = store.create(data)
            elif (previous := replays.get(idempotency_key)) is None:
                task = store.create(data)
                replays[idempotency_key] = (data.model_dump_json(), task)
            elif previous[0] != data.model_dump_json():
                raise ApiError(409, "Idempotency-Key was already used with a different request")
            else:
                task = previous[1]
                response.headers["Idempotency-Replayed"] = "true"
        response.headers["Location"] = f"/v1/tasks/{task.id}"
        return task

    @v1.get("/tasks/{task_id}")
    def read(task_id: int) -> Task:
        if (task := store.get(task_id)) is None:
            raise ApiError(404, f"task {task_id} not found")
        return task

    @v1.put("/tasks/{task_id}")
    def replace(task_id: int, data: TaskCreate) -> Task:
        # Naturally idempotent: sending the same body twice leaves the same state.
        if (task := store.replace(task_id, data)) is None:
            raise ApiError(404, f"task {task_id} not found")
        return task

    @v1.delete("/tasks/{task_id}", status_code=204)
    def delete(task_id: int) -> None:
        # Idempotent in effect (the task is gone); the repeat reports 404.
        if not store.delete(task_id):
            raise ApiError(404, f"task {task_id} not found")

    app.include_router(v1)
    return app


if __name__ == "__main__":
    serve(create_app())
