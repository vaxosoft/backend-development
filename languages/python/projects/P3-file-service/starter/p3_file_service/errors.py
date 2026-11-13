from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException

from .multipart_stream import MalformedUpload
from .storage import FileTooLarge


def error_response(status: int, code: str, message: str) -> JSONResponse:
    return JSONResponse(status_code=status, content={"error": {"code": code, "message": message}})


def register_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(HTTPException)
    async def http_error(_request: Request, exc: HTTPException) -> JSONResponse:
        code = {404: "not_found", 405: "method_not_allowed"}.get(exc.status_code, "http_error")
        return error_response(exc.status_code, code, str(exc.detail))

    @app.exception_handler(RequestValidationError)
    async def bad_request(_request: Request, _exc: RequestValidationError) -> JSONResponse:
        return error_response(400, "bad_request", "invalid request")

    @app.exception_handler(MalformedUpload)
    async def malformed(_request: Request, exc: MalformedUpload) -> JSONResponse:
        return error_response(400, "bad_request", str(exc))

    @app.exception_handler(FileTooLarge)
    async def too_large(_request: Request, _exc: FileTooLarge) -> JSONResponse:
        return error_response(413, "payload_too_large", "file exceeds the maximum upload size")
