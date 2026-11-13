"""Exercise 2: request-id middleware, panic recovery and CORS."""

from fastapi import FastAPI, Request


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
    raise NotImplementedError


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
