import asyncio
import logging
import os
import signal
import socket

from .app import create_app
from .server import run


def parse_addr(value: str) -> tuple[str, int]:
    host, _, port = value.rpartition(":")
    return host or "127.0.0.1", int(port)


async def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(message)s")
    host, port = parse_addr(os.environ.get("ADDR", "127.0.0.1:8080"))
    sock = socket.create_server((host, port))
    shutdown = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, shutdown.set)
    logging.getLogger("p1_hello_http").info("listening on %s:%d", host, port)
    await run(create_app(), sock, shutdown)
    logging.getLogger("p1_hello_http").info("stopped")


if __name__ == "__main__":
    asyncio.run(main())
