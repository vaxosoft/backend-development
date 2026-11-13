import logging

from fastapi import FastAPI, Query, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from .config import Config
from .models import Task, TaskCreate
from .store import InMemoryTaskStore, TaskStore

logger = logging.getLogger("p7_tested_service_docker_ci")


def error_response(
    status: int, code: str, message: str, fields: dict[str, str] | None = None
) -> JSONResponse:
    body: dict[str, object] = {"code": code, "message": message}
    if fields:
        body["fields"] = fields
    return JSONResponse({"error": body}, status_code=status)


def create_app(config: Config | None = None, store: TaskStore | None = None) -> FastAPI:
    config = config or Config()
    store = store or InMemoryTaskStore()
    app = FastAPI(title="Tasks service")
    app.state.config = config

    @app.exception_handler(RequestValidationError)
    async def on_invalid_request(_request: Request, exc: RequestValidationError) -> JSONResponse:
        errors = exc.errors()
        if any(e["type"] == "json_invalid" or e["loc"][0] == "query" for e in errors):
            return error_response(400, "bad_request", "malformed request")
        fields = {".".join(str(part) for part in e["loc"][1:]): e["msg"] for e in errors}
        return error_response(422, "validation_failed", "invalid task", fields)

    @app.exception_handler(StarletteHTTPException)
    async def on_http_error(_request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = "not_found" if exc.status_code == 404 else "bad_request"
        return error_response(exc.status_code, code, str(exc.detail))

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    # TODO(readiness): GET /readyz answers 503 {"status": "not_ready"} until startup has finished,
    #   then 200 {"status": "ready"}. A lifespan handler is the place to flip the flag.
    # TODO(metrics): GET /metrics answers text/plain with the line `requests_total <n>`, where n
    #   counts every request received (the scrape itself included). HTTP middleware can count.

    @app.post("/tasks", status_code=201)
    def create_task(data: TaskCreate, response: Response) -> Task:
        task = store.add(data)
        response.headers["Location"] = f"/tasks/{task.id}"
        return task

    @app.get("/tasks")
    def list_tasks(limit: int = Query(20, ge=1, le=100)) -> list[Task]:
        return store.list_all()[:limit]

    @app.get("/tasks/{task_id}")
    def get_task(task_id: int) -> Task:
        task = store.get(task_id)
        if task is None:
            raise StarletteHTTPException(404, "task not found")
        return task

    return app
