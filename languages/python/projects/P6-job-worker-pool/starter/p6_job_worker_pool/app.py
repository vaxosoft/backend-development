from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Annotated

from fastapi import FastAPI, Query, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from .config import Config
from .errors import InvalidPayload, JobFinished, JobNotFound, PoolClosed, QueueFull
from .handlers import validate_payload
from .models import JobAccepted, JobList, JobRequest, JobView
from .pool import WorkerPool

MAX_LIMIT = 100

CODES = {404: "not_found", 405: "method_not_allowed"}


def error_response(
    status: int,
    code: str,
    message: str,
    fields: dict[str, str] | None = None,
    headers: dict[str, str] | None = None,
) -> JSONResponse:
    body: dict[str, object] = {"code": code, "message": message}
    if fields:
        body["fields"] = fields
    return JSONResponse({"error": body}, status_code=status, headers=headers)


def create_app(config: Config | None = None) -> FastAPI:
    pool = WorkerPool(config or Config())

    @asynccontextmanager
    async def lifespan(_app: FastAPI) -> AsyncIterator[None]:
        await pool.start()
        try:
            yield
        finally:
            await pool.shutdown()

    app = FastAPI(title="Job worker pool", lifespan=lifespan)
    app.state.pool = pool

    @app.exception_handler(RequestValidationError)
    async def on_invalid_request(_request: Request, exc: RequestValidationError) -> JSONResponse:
        errors = exc.errors()
        if any(e["type"] == "json_invalid" or e["loc"][0] == "query" for e in errors):
            return error_response(400, "bad_request", "malformed request")
        fields = {".".join(str(part) for part in e["loc"][1:]): e["msg"] for e in errors}
        return error_response(422, "validation_failed", "invalid job", fields)

    @app.exception_handler(StarletteHTTPException)
    async def on_http_error(_request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = CODES.get(exc.status_code, "bad_request")
        return error_response(exc.status_code, code, str(exc.detail))

    @app.exception_handler(InvalidPayload)
    async def on_invalid_payload(_request: Request, exc: InvalidPayload) -> JSONResponse:
        return error_response(422, "validation_failed", "invalid job", exc.fields)

    @app.exception_handler(QueueFull)
    async def on_queue_full(_request: Request, _exc: QueueFull) -> JSONResponse:
        return error_response(
            429, "rate_limited", "job queue is full", headers={"Retry-After": "1"}
        )

    @app.exception_handler(PoolClosed)
    async def on_pool_closed(_request: Request, _exc: PoolClosed) -> JSONResponse:
        return error_response(503, "unavailable", "server is shutting down")

    @app.exception_handler(JobNotFound)
    async def on_not_found(_request: Request, _exc: JobNotFound) -> JSONResponse:
        return error_response(404, "not_found", "job not found")

    @app.exception_handler(JobFinished)
    async def on_finished(_request: Request, _exc: JobFinished) -> JSONResponse:
        return error_response(409, "conflict", "job already finished")

    # These handlers are `async def` on purpose: the pool is not thread-safe and must only be
    # touched from the event loop. A plain `def` handler would run in a worker thread.
    @app.post("/jobs", status_code=202)
    async def submit_job(request: JobRequest, response: Response) -> JobAccepted:
        validate_payload(request.type, request.payload)
        job = pool.submit(request.type, request.payload)
        response.headers["Location"] = f"/jobs/{job.id}"
        return JobAccepted(id=job.id)

    @app.get("/jobs")
    async def list_jobs(
        limit: Annotated[int, Query(ge=1)] = 20, offset: Annotated[int, Query(ge=0)] = 0
    ) -> JobList:
        limit = min(limit, MAX_LIMIT)
        jobs = pool.list_newest_first()
        page = jobs[offset : offset + limit]
        items = [JobView.model_validate(job) for job in page]
        return JobList(items=items, total=len(jobs), limit=limit, offset=offset)

    @app.get("/jobs/{job_id}")
    async def get_job(job_id: str) -> JobView:
        job = pool.get(job_id)
        if job is None:
            raise JobNotFound
        return JobView.model_validate(job)

    @app.delete("/jobs/{job_id}", status_code=204)
    async def cancel_job(job_id: str) -> Response:
        pool.cancel(job_id)
        return Response(status_code=204)

    return app
