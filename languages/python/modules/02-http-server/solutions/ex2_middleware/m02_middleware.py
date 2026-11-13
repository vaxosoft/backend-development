"""Exercise 2: request-id middleware, panic recovery and CORS."""

import logging
import re
import time
import uuid
from collections.abc import Awaitable, Callable

from fastapi import FastAPI, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

logger = logging.getLogger(__name__)

REQUEST_ID = re.compile(r"[A-Za-z0-9_-]{1,64}")


def install_middleware(app: FastAPI) -> None:
    """Add the middleware stack to `app`:

    1. Request id: use the incoming X-Request-ID when it matches ^[A-Za-z0-9_-]{1,64}$, otherwise
       generate uuid4().hex. Store it in `request.state.request_id` and return it in an
       X-Request-ID header on EVERY response (200, 404 and 500).
    2. Timing: an X-Process-Time-Ms header with the elapsed milliseconds as a number.
    3. Recovery: an unhandled exception becomes status 500 with the body
       {"error": {"code": "internal", "message": "internal server error"}}.
       Never leak the exception text; the app must keep serving afterwards.
    4. CORS: allow only the origin https://app.example.com, for GET and POST, with the request
       headers Content-Type and X-Request-ID, and let browsers read X-Request-ID.
    """

    @app.middleware("http")
    async def request_context(
        request: Request, call_next: Callable[[Request], Awaitable[Response]]
    ) -> Response:
        incoming = request.headers.get("X-Request-ID", "")
        request_id = incoming if REQUEST_ID.fullmatch(incoming) else uuid.uuid4().hex
        request.state.request_id = request_id
        started = time.perf_counter()
        try:
            response = await call_next(request)
        except Exception:
            logger.exception("unhandled error", extra={"request_id": request_id})
            response = JSONResponse(
                {"error": {"code": "internal", "message": "internal server error"}},
                status_code=500,
            )
        response.headers["X-Request-ID"] = request_id
        response.headers["X-Process-Time-Ms"] = f"{(time.perf_counter() - started) * 1000:.3f}"
        return response

    # Added last, so it is the outermost layer and also decorates the 500 built above.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["https://app.example.com"],
        allow_methods=["GET", "POST"],
        allow_headers=["Content-Type", "X-Request-ID"],
        expose_headers=["X-Request-ID"],
    )


def create_app() -> FastAPI:
    app = FastAPI()

    @app.get("/ok")
    def ok() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/whoami")
    def whoami(request: Request) -> dict[str, str]:
        return {"request_id": request.state.request_id}

    @app.get("/boom")
    def boom() -> dict[str, str]:
        raise RuntimeError("secret internal detail")

    install_middleware(app)
    return app
