"""Task Tracker service entry point with production-minded runtime settings.

Run:  python modules/14-packaging-and-running/examples/04_task_service.py
Env:  ADDR (host:port, default 127.0.0.1:8080), APP_ENV, LOG_LEVEL, SHUTDOWN_TIMEOUT,
      APP_VERSION, GIT_COMMIT (normally injected at build time)
"""

import os
import signal
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager, suppress
from typing import Any

import uvicorn
from fastapi import FastAPI, Response

TASKS = [
    {"id": 1, "title": "Write docs", "status": "todo", "tags": ["docs"]},
    {"id": 2, "title": "Ship v1", "status": "in_progress", "tags": []},
]


def split_addr(addr: str) -> tuple[str, int]:
    host, _, port = addr.rpartition(":")
    return host or "127.0.0.1", int(port)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    app.state.ready = True  # warm caches, open the database, ... then flip to ready
    yield
    app.state.ready = False


def create_app() -> FastAPI:
    app = FastAPI(title="Task Tracker", lifespan=lifespan)
    app.state.ready = False

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/readyz")
    def readyz(response: Response) -> dict[str, str]:
        if not app.state.ready:
            response.status_code = 503
            return {"status": "starting"}
        return {"status": "ready"}

    @app.get("/version")
    def version() -> dict[str, str]:
        return {
            "version": os.environ.get("APP_VERSION", "dev"),
            "commit": os.environ.get("GIT_COMMIT", "unknown"),
            "env": os.environ.get("APP_ENV", "development"),
        }

    @app.get("/tasks")
    def list_tasks() -> dict[str, Any]:
        return {"items": TASKS, "total": len(TASKS), "limit": 20, "offset": 0}

    return app


def main() -> None:
    host, port = split_addr(os.environ.get("ADDR", "127.0.0.1:8080"))
    config = uvicorn.Config(
        create_app(),
        host=host,
        port=port,
        log_level=os.environ.get("LOG_LEVEL", "info").lower(),
        timeout_graceful_shutdown=int(os.environ.get("SHUTDOWN_TIMEOUT", "10")),
        server_header=False,
    )
    # After a graceful shutdown uvicorn re-raises the signal it caught, so the process would
    # end with status 143 (SIGTERM) or a KeyboardInterrupt traceback (SIGINT). Exit with 0.
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    with suppress(KeyboardInterrupt):
        uvicorn.Server(config).run()


if __name__ == "__main__":
    main()
