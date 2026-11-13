"""Exercise 3: access-log and recovery middleware (pure ASGI).

`AccessLogMiddleware(app, logger, clock=..., id_factory=...)` must, for every HTTP request:

1. Use the incoming `X-Request-ID` header when it matches `[A-Za-z0-9._-]{1,64}`,
   otherwise call `id_factory()`. Add the id to the response as `X-Request-ID`.
2. Read `clock()` once before and once after the app runs. Log exactly ONE INFO record
   (message "request") on `logger` with `extra=` fields `request_id`, `method`, `path`,
   `status` and `duration_ms` (seconds between the two clock reads x 1000, rounded to 2 decimals).
3. If the app raises, log the traceback with `logger.exception("unhandled error", ...)`
   (include `request_id`) and answer 500 with the JSON envelope
   `{"error": {"code": "internal", "message": "internal server error"}}`; the access
   record then has status 500 and the exception text is never sent to the client.
4. Pass non-HTTP scopes (lifespan, websocket) straight through.
"""

import logging
import re
import time
import uuid
from collections.abc import Callable

from starlette.datastructures import Headers, MutableHeaders
from starlette.responses import JSONResponse
from starlette.types import ASGIApp, Message, Receive, Scope, Send

VALID_REQUEST_ID = re.compile(r"[A-Za-z0-9._-]{1,64}")


def _default_id() -> str:
    return uuid.uuid4().hex


class AccessLogMiddleware:
    def __init__(
        self,
        app: ASGIApp,
        logger: logging.Logger,
        clock: Callable[[], float] = time.perf_counter,
        id_factory: Callable[[], str] = _default_id,
    ) -> None:
        self.app = app
        self.logger = logger
        self.clock = clock
        self.id_factory = id_factory

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        incoming = Headers(scope=scope).get("x-request-id", "")
        request_id = incoming if VALID_REQUEST_ID.fullmatch(incoming) else self.id_factory()
        started = self.clock()
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
            self.logger.exception("unhandled error", extra={"request_id": request_id})
            if not response_started:
                body = {"error": {"code": "internal", "message": "internal server error"}}
                await JSONResponse(body, status_code=500)(scope, receive, send_with_id)
        finally:
            self.logger.info(
                "request",
                extra={
                    "request_id": request_id,
                    "method": scope["method"],
                    "path": scope["path"],
                    "status": status,
                    "duration_ms": round((self.clock() - started) * 1000, 2),
                },
            )
