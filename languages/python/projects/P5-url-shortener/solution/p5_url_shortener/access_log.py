"""Structured access log: one JSON line per request."""

import json
import re
import time
import uuid
from collections.abc import Awaitable, Callable

from fastapi import FastAPI, Request, Response

from .config import Config

REQUEST_ID_HEADER = "X-Request-ID"
# an unchecked id would let a client forge log content
_SAFE_REQUEST_ID = re.compile(r"[A-Za-z0-9._-]{1,64}")


def install_access_log(app: FastAPI, config: Config) -> None:
    """Add middleware that sends one JSON line per request to `config.log_sink`.

    Fields: request_id, method, path, status, duration_ms and (when known) user_id.
    Also answer with an `X-Request-ID` header: echo a sane incoming value, else generate one.
    Never log request bodies, query strings or the Authorization header.
    """

    @app.middleware("http")
    async def access_log(
        request: Request, call_next: Callable[[Request], Awaitable[Response]]
    ) -> Response:
        incoming = request.headers.get(REQUEST_ID_HEADER, "")
        request_id = incoming if _SAFE_REQUEST_ID.fullmatch(incoming) else uuid.uuid4().hex
        started = time.perf_counter()
        status = 500
        try:
            response = await call_next(request)
            status = response.status_code
            response.headers[REQUEST_ID_HEADER] = request_id
            return response
        finally:
            record: dict[str, object] = {
                "request_id": request_id,
                "method": request.method,
                "path": request.url.path,
                "status": status,
                "duration_ms": round((time.perf_counter() - started) * 1000, 3),
            }
            user_id = getattr(request.state, "user_id", None)
            if user_id is not None:
                record["user_id"] = user_id
            config.log_sink(json.dumps(record))
