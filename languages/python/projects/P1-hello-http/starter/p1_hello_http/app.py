import asyncio
import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse, PlainTextResponse
from starlette.exceptions import HTTPException

from .errors import error_response
from .middleware import access_log, recover, request_id

logger = logging.getLogger("p1_hello_http")

MAX_SLOW_MS = 2000

type Sink = Callable[[str], None]
type Sleep = Callable[[float], Awaitable[None]]


@dataclass(slots=True, frozen=True)
class Config:
    """Everything the tests need to replace: where log lines go and how /slow waits."""

    log_sink: Sink = logger.info
    sleep: Sleep = asyncio.sleep


def create_app(config: Config | None = None) -> FastAPI:
    config = config or Config()
    app = FastAPI(title="P1 Hello HTTP")

    # The middleware added last is the outermost one: request id -> access log -> recover -> route.
    app.middleware("http")(recover)
    app.middleware("http")(access_log(config.log_sink))
    app.middleware("http")(request_id)
    register_error_handlers(app)

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/hello", response_class=PlainTextResponse)
    async def hello_world() -> str:
        # TODO: greet "world": Hello, world!
        raise NotImplementedError

    @app.get("/hello/{name}", response_class=PlainTextResponse)
    async def hello(name: str) -> str:
        # TODO: Hello, {name}!
        raise NotImplementedError

    @app.get("/slow", response_class=PlainTextResponse)
    async def slow(ms: int = 100) -> str:
        # TODO: accept only 0..MAX_SLOW_MS (otherwise the 400 envelope), wait with
        # `await config.sleep(seconds)` and answer "done".
        raise NotImplementedError

    @app.get("/panic")
    async def panic() -> None:
        # TODO: fail with an unexpected exception; the recover middleware must turn it into a 500.
        raise NotImplementedError

    return app


def register_error_handlers(app: FastAPI) -> None:
    @app.exception_handler(RequestValidationError)
    async def bad_query(_request: Request, exc: RequestValidationError) -> JSONResponse:
        first = exc.errors()[0]
        name = ".".join(str(part) for part in first["loc"])
        return error_response(400, "bad_request", f"invalid {name}: {first['msg']}")

    @app.exception_handler(HTTPException)
    async def http_error(_request: Request, exc: HTTPException) -> JSONResponse:
        code = {404: "not_found", 405: "method_not_allowed"}.get(exc.status_code, "http_error")
        return error_response(exc.status_code, code, str(exc.detail))
