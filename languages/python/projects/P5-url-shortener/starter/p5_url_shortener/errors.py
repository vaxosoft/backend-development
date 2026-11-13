"""The shared error envelope: {"error": {"code", "message", "fields"?}}."""

from collections.abc import Mapping
from typing import Any

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException


class AppError(Exception):
    def __init__(
        self,
        status: int,
        code: str,
        message: str,
        fields: dict[str, str] | None = None,
        headers: Mapping[str, str] | None = None,
    ) -> None:
        super().__init__(message)
        self.status = status
        self.code = code
        self.message = message
        self.fields = fields
        self.headers = headers


def error_response(error: AppError) -> JSONResponse:
    body: dict[str, Any] = {"code": error.code, "message": error.message}
    if error.fields:
        body["fields"] = error.fields
    return JSONResponse({"error": body}, status_code=error.status, headers=error.headers)


_CODES = {
    400: "bad_request",
    401: "unauthorized",
    403: "forbidden",
    404: "not_found",
    405: "method_not_allowed",
    409: "conflict",
    413: "payload_too_large",
}


def install_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(AppError)
    async def handle_app_error(_request: Request, exc: AppError) -> JSONResponse:
        return error_response(exc)

    @app.exception_handler(StarletteHTTPException)
    async def handle_http_error(_request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = _CODES.get(exc.status_code, "error")
        return error_response(AppError(exc.status_code, code, str(exc.detail), headers=exc.headers))

    @app.exception_handler(RequestValidationError)
    async def handle_validation_error(
        _request: Request, exc: RequestValidationError
    ) -> JSONResponse:
        errors = exc.errors()
        if any(e["type"] == "json_invalid" for e in errors):
            return error_response(AppError(400, "bad_request", "malformed JSON body"))
        if any(e["loc"][0] == "query" for e in errors):
            return error_response(AppError(400, "bad_request", "invalid query parameter"))
        fields = {str(e["loc"][-1]): e["msg"].removeprefix("Value error, ") for e in errors}
        return error_response(AppError(422, "validation_failed", "invalid request", fields))

    @app.exception_handler(Exception)
    async def handle_unexpected(_request: Request, _exc: Exception) -> JSONResponse:
        # never leak details; the traceback belongs in the server log
        return error_response(AppError(500, "internal", "internal server error"))
