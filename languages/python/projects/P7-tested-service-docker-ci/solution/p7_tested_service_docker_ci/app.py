import logging
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager

from fastapi import FastAPI, Query, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, PlainTextResponse
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

    @asynccontextmanager
    async def lifespan(application: FastAPI) -> AsyncIterator[None]:
        application.state.ready = True
        logger.info("ready (env=%s)", config.env)
        yield
        application.state.ready = False

    app = FastAPI(title="Tasks service", lifespan=lifespan)
    app.state.config = config
    app.state.ready = False
    app.state.requests_total = 0

    @app.middleware("http")
    async def count_requests(
        request: Request, call_next: Callable[[Request], Awaitable[Response]]
    ) -> Response:
        app.state.requests_total += 1
        return await call_next(request)

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

    @app.get("/readyz")
    def readyz() -> JSONResponse:
        if app.state.ready:
            return JSONResponse({"status": "ready"})
        return JSONResponse({"status": "not_ready"}, status_code=503)

    @app.get("/metrics", response_class=PlainTextResponse)
    def metrics() -> str:
        return f"requests_total {app.state.requests_total}\n"

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
