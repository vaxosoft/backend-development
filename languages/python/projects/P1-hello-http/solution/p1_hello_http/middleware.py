import logging
import re
import time
import uuid
from collections.abc import Awaitable, Callable
from urllib.parse import quote

from starlette.middleware.base import RequestResponseEndpoint
from starlette.requests import Request
from starlette.responses import Response

from .errors import error_response

logger = logging.getLogger("p1_hello_http")

REQUEST_ID_HEADER = "X-Request-ID"
_SAFE_ID = re.compile(r"[A-Za-z0-9._-]{1,64}")

type Middleware = Callable[[Request, RequestResponseEndpoint], Awaitable[Response]]
type Sink = Callable[[str], None]


async def request_id(request: Request, call_next: RequestResponseEndpoint) -> Response:
    """Echo a safe incoming X-Request-ID or generate one; expose it as request.state.request_id."""
    incoming = request.headers.get(REQUEST_ID_HEADER, "")
    rid = incoming if _SAFE_ID.fullmatch(incoming) else uuid.uuid4().hex
    request.state.request_id = rid
    response = await call_next(request)
    response.headers[REQUEST_ID_HEADER] = rid
    return response


def access_log(sink: Sink) -> Middleware:
    """Build a middleware that sends one line per request to `sink`."""

    async def middleware(request: Request, call_next: RequestResponseEndpoint) -> Response:
        started = time.perf_counter()
        response = await call_next(request)
        duration_ms = (time.perf_counter() - started) * 1000
        rid = getattr(request.state, "request_id", "-")
        sink(
            f"method={request.method} path={quote(request.url.path)} "
            f"status={response.status_code} duration_ms={duration_ms:.2f} request_id={rid}"
        )
        return response

    return middleware


async def recover(request: Request, call_next: RequestResponseEndpoint) -> Response:
    """Turn an unhandled exception into the 500 error envelope so the server keeps serving."""
    try:
        return await call_next(request)
    except Exception:
        logger.exception("unhandled error in %s %s", request.method, quote(request.url.path))
        return error_response(500, "internal", "internal server error")
