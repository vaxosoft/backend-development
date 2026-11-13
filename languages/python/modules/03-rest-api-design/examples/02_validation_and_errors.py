"""Validation and one consistent error envelope for every kind of failure.

Run:  FIXED_NOW=2030-01-01T09:00:00Z python 02_validation_and_errors.py   (ADDR=host:port)
"""

from fastapi import FastAPI, Response
from m03_errors import ApiError, install_error_handlers, serve
from m03_tasks import Task, TaskCreate, TaskStore, clock_from_env


def create_app() -> FastAPI:
    app = FastAPI(title="Validation demo")
    install_error_handlers(app)
    store = TaskStore(clock_from_env())

    @app.post("/tasks", status_code=201)
    def create(data: TaskCreate, response: Response) -> Task:
        task = store.create(data)
        response.headers["Location"] = f"/tasks/{task.id}"
        return task

    @app.get("/tasks/{task_id}")
    def read(task_id: int) -> Task:
        task = store.get(task_id)
        if task is None:
            raise ApiError(404, f"task {task_id} not found")
        return task

    @app.get("/boom")
    def boom() -> None:
        raise RuntimeError("database password is hunter2")

    return app


if __name__ == "__main__":
    serve(create_app())
