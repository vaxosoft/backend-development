"""Task Tracker service with request ids, one JSON access-log line per request and crash recovery.

Run:  python modules/10-logging-and-diagnostics/examples/05_tracker_service.py
Env:  ADDR (default 127.0.0.1:8080), LOG_LEVEL (default INFO)
"""

import logging
import os
import re
import sys
import time
import uuid
from dataclasses import dataclass

import uvicorn
from fastapi import FastAPI, Response
from fastapi.responses import JSONResponse, PlainTextResponse
from m10_helpers import JsonFormatter, RequestIdFilter, request_id_var
from pydantic import BaseModel, Field
from starlette.datastructures import Headers, MutableHeaders
from starlette.types import ASGIApp, Message, Receive, Scope, Send

access_log = logging.getLogger("tracker.access")
log = logging.getLogger("tracker.api")

VALID_REQUEST_ID = re.compile(r"[A-Za-z0-9._-]{1,64}")


@dataclass
class Metrics:
    requests_total: int = 0
    errors_total: int = 0  # responses with status >= 500


metrics = Metrics()


class RequestLogMiddleware:
    """Pure ASGI middleware: it sees every request and keeps ContextVars working."""

    def __init__(self, app: ASGIApp) -> None:
        self.app = app

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        incoming = Headers(scope=scope).get("x-request-id", "")
        request_id = incoming if VALID_REQUEST_ID.fullmatch(incoming) else uuid.uuid4().hex
        token = request_id_var.set(request_id)
        started = time.perf_counter()
        status = 500
        response_started = False

        async def send_with_id(message: Message) -> None:
            nonlocal status, response_started
            if message["type"] == "http.response.start":
                status = message["status"]
                response_started = True
                MutableHeaders(scope=message)["X-Request-ID"] = request_id
            await send(message)

        try:
            await self.app(scope, receive, send_with_id)
        except Exception:
            log.exception("unhandled error")  # full traceback goes to the log only
            if not response_started:
                status = 500
                body = {"error": {"code": "internal", "message": "internal server error"}}
                await JSONResponse(body, status_code=500)(scope, receive, send_with_id)
        finally:
            metrics.requests_total += 1
            metrics.errors_total += status >= 500
            access_log.info(
                "request",
                extra={
                    "method": scope["method"],
                    "path": scope["path"],
                    "status": status,
                    "duration_ms": round((time.perf_counter() - started) * 1000, 2),
                },
            )
            request_id_var.reset(token)


class TaskIn(BaseModel):
    title: str = Field(min_length=1, max_length=200)


def create_app() -> FastAPI:
    app = FastAPI()
    app.add_middleware(RequestLogMiddleware)
    tasks: list[dict[str, object]] = []

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/metrics")
    def metrics_text() -> Response:
        text = f"requests_total {metrics.requests_total}\nerrors_total {metrics.errors_total}\n"
        return PlainTextResponse(text)

    @app.post("/tasks", status_code=201)
    def create_task(body: TaskIn) -> dict[str, object]:
        task: dict[str, object] = {"id": len(tasks) + 1, "title": body.title, "status": "todo"}
        tasks.append(task)
        log.info("task created", extra={"task_id": task["id"]})
        return task

    @app.get("/panic")
    def panic() -> None:
        raise RuntimeError("boom: secret internal detail")

    return app


def configure_logging(level: str) -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.addFilter(RequestIdFilter())
    handler.setFormatter(JsonFormatter())
    logging.basicConfig(level=level, handlers=[handler], force=True)


def main() -> None:
    configure_logging(os.environ.get("LOG_LEVEL", "INFO"))
    host, _, port = os.environ.get("ADDR", "127.0.0.1:8080").rpartition(":")
    # log_config=None: uvicorn's records flow into our root handler, as JSON.
    # access_log=False: our middleware already writes the access line.
    uvicorn.run(create_app(), host=host, port=int(port), log_config=None, access_log=False)


if __name__ == "__main__":
    main()
