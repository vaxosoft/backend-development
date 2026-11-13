import asyncio
import contextlib
import socket
from collections.abc import Iterator

import uvicorn
from fastapi import FastAPI


class _Server(uvicorn.Server):
    """uvicorn.Server that leaves SIGINT/SIGTERM to the caller (see __main__)."""

    @contextlib.contextmanager
    def capture_signals(self) -> Iterator[None]:
        yield


def make_server(app: FastAPI) -> uvicorn.Server:
    config = uvicorn.Config(
        app,
        log_config=None,
        access_log=False,
        log_level="warning",
        timeout_keep_alive=5,
        timeout_graceful_shutdown=10,
    )
    return _Server(config)


async def run(app: FastAPI, sock: socket.socket, shutdown: asyncio.Event) -> None:
    """Serve on `sock` until `shutdown` is set, then stop gracefully.

    In-flight requests finish, the listening socket is closed, and run() returns afterwards.
    """
    server = make_server(app)
    serving = asyncio.create_task(server.serve([sock]))
    stop = asyncio.create_task(shutdown.wait())
    try:
        await asyncio.wait({serving, stop}, return_when=asyncio.FIRST_COMPLETED)
        server.should_exit = True
        await serving
    finally:
        stop.cancel()
