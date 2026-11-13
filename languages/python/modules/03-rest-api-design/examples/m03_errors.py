"""Shared by the module 03 examples: the error envelope and the handlers that produce it.

Every error response looks like
    {"error": {"code": "validation_failed", "message": "invalid request", "fields": {...}}}
"""

import logging
import os
from typing import Any

import uvicorn
from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

logger = logging.getLogger("api")

STATUS_CODES = {
    400: "bad_request",
    401: "unauthorized",
    403: "forbidden",
    404: "not_found",
    405: "method_not_allowed",
    409: "conflict",
    413: "payload_too_large",
    422: "validation_failed",
    429: "rate_limited",
}


class ApiError(Exception):
    """Raise from any handler or dependency to produce an envelope response."""

    def __init__(
        self,
        status: int,
        message: str,
        *,
        code: str | None = None,
        fields: dict[str, str] | None = None,
        headers: dict[str, str] | None = None,
    ) -> None:
        super().__init__(message)
        self.status = status
        self.message = message
        self.code = code or STATUS_CODES.get(status, "internal" if status >= 500 else "error")
        self.fields = fields
        self.headers = headers


def envelope(
    status: int, code: str, message: str, fields: dict[str, str] | None = None
) -> JSONResponse:
    body: dict[str, Any] = {"code": code, "message": message}
    if fields:
        body["fields"] = fields
    return JSONResponse({"error": body}, status_code=status)


def field_errors(exc: RequestValidationError) -> dict[str, str]:
    """Map Pydantic error locations like ("body", "tags", 0) to {"tags.0": message}."""
    fields: dict[str, str] = {}
    for err in exc.errors():
        loc = [str(part) for part in err["loc"][1:]]
        fields.setdefault(".".join(loc) or "body", err["msg"].removeprefix("Value error, "))
    return fields


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(ApiError)
    async def on_api_error(request: Request, exc: ApiError) -> JSONResponse:
        response = envelope(exc.status, exc.code, exc.message, exc.fields)
        response.headers.update(exc.headers or {})
        return response

    @app.exception_handler(StarletteHTTPException)
    async def on_http_error(request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = STATUS_CODES.get(exc.status_code, "internal" if exc.status_code >= 500 else "error")
        response = envelope(exc.status_code, code, str(exc.detail))
        response.headers.update(dict(exc.headers or {}))
        return response

    @app.exception_handler(RequestValidationError)
    async def on_validation_error(request: Request, exc: RequestValidationError) -> JSONResponse:
        errors = exc.errors()
        # Broken JSON, and bad path/query/header values, are bad requests; only a well-formed
        # body with invalid data is "unprocessable".
        if any(e["type"] == "json_invalid" for e in errors):
            return envelope(400, "bad_request", "malformed JSON body")
        if any(e["loc"][0] != "body" for e in errors):
            return envelope(400, "bad_request", "invalid request parameters", field_errors(exc))
        return envelope(422, "validation_failed", "invalid request", field_errors(exc))

    @app.exception_handler(Exception)
    async def on_unexpected(request: Request, exc: Exception) -> JSONResponse:
        logger.error("unhandled error on %s %s", request.method, request.url.path, exc_info=exc)
        return envelope(500, "internal", "internal server error")


def serve(app: FastAPI) -> None:
    host, _, port = os.environ.get("ADDR", "127.0.0.1:8080").rpartition(":")
    uvicorn.run(app, host=host or "127.0.0.1", port=int(port), timeout_graceful_shutdown=10)
