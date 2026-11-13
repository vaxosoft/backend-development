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
    # TODO: configure server timeouts: keep-alive of at most 30 s and a bounded
    # timeout_graceful_shutdown, so a stuck client cannot block a shutdown forever.
    config = uvicorn.Config(app, log_config=None, access_log=False, log_level="warning")
    return _Server(config)


async def run(app: FastAPI, sock: socket.socket, shutdown: asyncio.Event) -> None:
    """Serve on `sock` until `shutdown` is set, then stop gracefully.

    In-flight requests finish, the listening socket is closed, and run() returns afterwards.
    """
    # TODO: start make_server(app).serve([sock]) as a task, wait for `shutdown` (or for the
    # server to stop), set server.should_exit = True and await the server task.
    raise NotImplementedError
