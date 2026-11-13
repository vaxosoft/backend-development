from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException


def error_response(
    status: int, code: str, message: str, fields: dict[str, str] | None = None
) -> JSONResponse:
    """The shared envelope: {"error": {"code", "message", "fields"?}}."""
    error: dict[str, object] = {"code": code, "message": message}
    if fields:
        error["fields"] = fields
    return JSONResponse(status_code=status, content={"error": error})


def register_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(HTTPException)
    async def http_error(_request: Request, exc: HTTPException) -> JSONResponse:
        code = {404: "not_found", 405: "method_not_allowed"}.get(exc.status_code, "http_error")
        return error_response(exc.status_code, code, str(exc.detail))

    # TODO: FastAPI answers invalid input with its own {"detail": [...]} body and always 422.
    # Register a handler for RequestValidationError that produces the shared envelope:
    #   - malformed JSON (error type "json_invalid")      -> 400 bad_request
    #   - a bad query parameter (loc starts with "query") -> 400 bad_request
    #   - a bad path parameter, e.g. /tasks/abc           -> 404 not_found
    #   - invalid body fields                             -> 422 validation_failed + fields
    # PATCH validates by hand (see app.py) and raises pydantic.ValidationError: handle it too.
