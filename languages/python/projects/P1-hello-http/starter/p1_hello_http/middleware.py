import logging
from collections.abc import Awaitable, Callable

from starlette.middleware.base import RequestResponseEndpoint
from starlette.requests import Request
from starlette.responses import Response

logger = logging.getLogger("p1_hello_http")

REQUEST_ID_HEADER = "X-Request-ID"

type Middleware = Callable[[Request, RequestResponseEndpoint], Awaitable[Response]]
type Sink = Callable[[str], None]


async def request_id(request: Request, call_next: RequestResponseEndpoint) -> Response:
    """Echo a safe incoming X-Request-ID or generate one; expose it as request.state.request_id."""
    # TODO: accept the incoming id only if it is 1-64 characters of [A-Za-z0-9._-],
    # otherwise generate a new one; store it on request.state and add it to the response.
    return await call_next(request)


def access_log(sink: Sink) -> Middleware:
    """Build a middleware that sends one line per request to `sink`."""

    async def middleware(request: Request, call_next: RequestResponseEndpoint) -> Response:
        # TODO: time the request and call sink() once with a single line:
        # method=GET path=/healthz status=200 duration_ms=0.42 request_id=<id>
        # Percent-encode the path (urllib.parse.quote) so a client cannot inject new lines.
        return await call_next(request)

    return middleware


async def recover(request: Request, call_next: RequestResponseEndpoint) -> Response:
    """Turn an unhandled exception into the 500 error envelope so the server keeps serving."""
    # TODO: catch Exception, log it with logger.exception and return
    # error_response(500, "internal", "internal server error") without leaking details.
    return await call_next(request)
