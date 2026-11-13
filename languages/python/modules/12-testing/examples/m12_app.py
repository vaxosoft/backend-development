"""A small FastAPI app over `TaskService`, with the service injected via `Depends`."""

from datetime import datetime
from functools import lru_cache
from typing import Annotated

from fastapi import Depends, FastAPI, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from m12_tasks import InMemoryTaskRepository, NotFoundError, Task, TaskService, ValidationError
from pydantic import BaseModel


class TaskIn(BaseModel):
    title: str
    tags: list[str] = []


class TaskOut(BaseModel):
    id: int
    title: str
    status: str
    tags: list[str]
    created_at: datetime

    @classmethod
    def of(cls, task: Task) -> TaskOut:
        return cls(
            id=task.id,
            title=task.title,
            status=task.status,
            tags=list(task.tags),
            created_at=task.created_at,
        )


@lru_cache
def get_service() -> TaskService:
    """Production wiring. Tests replace this with `app.dependency_overrides`."""
    return TaskService(InMemoryTaskRepository())


Service = Annotated[TaskService, Depends(get_service)]


def error(status: int, code: str, message: str) -> JSONResponse:
    return JSONResponse({"error": {"code": code, "message": message}}, status_code=status)


def create_app() -> FastAPI:
    app = FastAPI()

    @app.exception_handler(NotFoundError)
    def not_found(_: Request, exc: NotFoundError) -> JSONResponse:
        return error(404, "not_found", str(exc))

    @app.exception_handler(ValidationError)
    def invalid(_: Request, exc: ValidationError) -> JSONResponse:
        return error(422, "validation_failed", str(exc))

    @app.exception_handler(RequestValidationError)
    def bad_body(_: Request, exc: RequestValidationError) -> JSONResponse:
        if any(e["type"] == "json_invalid" for e in exc.errors()):
            return error(400, "bad_request", "malformed JSON")
        return error(422, "validation_failed", "invalid request body")

    @app.post("/tasks", status_code=201)
    def create_task(body: TaskIn, service: Service, response: Response) -> TaskOut:
        task = service.create(body.title, body.tags)
        response.headers["Location"] = f"/tasks/{task.id}"
        return TaskOut.of(task)

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int, service: Service) -> TaskOut:
        return TaskOut.of(service.get(task_id))

    @app.get("/tasks")
    def list_tasks(service: Service) -> list[TaskOut]:
        return [TaskOut.of(task) for task in service.list_all()]

    return app
