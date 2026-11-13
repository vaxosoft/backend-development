"""Shared by the module 02 examples: read ADDR and run an ASGI app with uvicorn."""

import os

import uvicorn
from fastapi import FastAPI

DEFAULT_ADDR = "127.0.0.1:8080"


def parse_addr(raw: str) -> tuple[str, int]:
    host, _, port = raw.rpartition(":")
    return host or "127.0.0.1", int(port)


def serve(app: FastAPI) -> None:
    """Block until Ctrl-C or SIGTERM; uvicorn stops accepting and drains in-flight requests."""
    host, port = parse_addr(os.environ.get("ADDR", DEFAULT_ADDR))
    uvicorn.run(
        app,
        host=host,
        port=port,
        log_level="info",
        timeout_keep_alive=5,
        timeout_graceful_shutdown=10,
    )
