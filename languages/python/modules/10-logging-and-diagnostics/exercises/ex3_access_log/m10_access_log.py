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
import time
import uuid
from collections.abc import Callable

from starlette.types import ASGIApp, Receive, Scope, Send


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
        raise NotImplementedError
