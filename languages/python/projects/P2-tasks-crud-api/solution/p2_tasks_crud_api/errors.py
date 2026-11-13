from collections.abc import Sequence
from typing import Any

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import ValidationError
from starlette.exceptions import HTTPException


def error_response(
    status: int, code: str, message: str, fields: dict[str, str] | None = None
) -> JSONResponse:
    """The shared envelope: {"error": {"code", "message", "fields"?}}."""
    error: dict[str, object] = {"code": code, "message": message}
    if fields:
        error["fields"] = fields
    return JSONResponse(status_code=status, content={"error": error})


def _fields(errors: Sequence[Any], skip: int) -> dict[str, str]:
    """Map pydantic errors to {field: message}; the first error of each field wins."""
    fields: dict[str, str] = {}
    for err in errors:
        loc = err["loc"][skip:]
        name = str(loc[0]) if loc else "body"
        fields.setdefault(name, err["msg"].removeprefix("Value error, "))
    return fields


def register_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(HTTPException)
    async def http_error(_request: Request, exc: HTTPException) -> JSONResponse:
        code = {404: "not_found", 405: "method_not_allowed"}.get(exc.status_code, "http_error")
        return error_response(exc.status_code, code, str(exc.detail))

    @app.exception_handler(RequestValidationError)
    async def request_invalid(_request: Request, exc: RequestValidationError) -> JSONResponse:
        errors = exc.errors()
        if any(err["type"] == "json_invalid" for err in errors):
            return error_response(400, "bad_request", "malformed JSON body")
        source = errors[0]["loc"][0]
        if source == "path":
            return error_response(404, "not_found", "task not found")
        if source == "query":
            return error_response(
                400, "bad_request", f"invalid query parameter: {errors[0]['loc'][1]}"
            )
        return error_response(422, "validation_failed", "invalid task", _fields(errors, skip=1))

    @app.exception_handler(ValidationError)
    async def model_invalid(_request: Request, exc: ValidationError) -> JSONResponse:
        return error_response(422, "validation_failed", "invalid task", _fields(exc.errors(), 0))
