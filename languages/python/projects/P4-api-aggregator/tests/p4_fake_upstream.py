"""A fake upstream that counts concurrent calls and injects failures and delays."""

import asyncio
import socket
import threading
import time
from collections import Counter
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass

import uvicorn
from fastapi import FastAPI, HTTPException, Request
from fastapi.testclient import TestClient

MakeClient = Callable[..., TestClient]


@dataclass(slots=True)
class Behaviour:
    status: int = 200
    fail_first: int = 0  # answer 500 this many times before behaving normally
    delay: float = 0.0


class FakeUpstream:
    def __init__(self) -> None:
        self.app = FastAPI()
        self.app.add_api_route("/items/{item_id}", self._get_item)
        self.in_flight = 0
        self.reset()

    def reset(self) -> None:
        """Forget behaviours and counters once calls abandoned by a previous test have ended."""
        deadline = time.monotonic() + 2
        while self.in_flight and time.monotonic() < deadline:
            time.sleep(0.005)
        self.behaviour: dict[int, Behaviour] = {}
        self.default_delay = 0.0
        self.calls: Counter[int] = Counter()
        self.in_flight = 0
        self.max_in_flight = 0
        self.abandoned = 0  # calls whose client went away before the answer

    async def _get_item(self, item_id: int, request: Request) -> dict[str, object]:
        self.calls[item_id] += 1
        attempt = self.calls[item_id]
        rule = self.behaviour.get(item_id, Behaviour())
        self.in_flight += 1
        self.max_in_flight = max(self.max_in_flight, self.in_flight)
        try:
            if await self._sleep_unless_abandoned(rule.delay or self.default_delay, request):
                self.abandoned += 1
        finally:
            self.in_flight -= 1
        if attempt <= rule.fail_first:
            raise HTTPException(status_code=500, detail="injected")
        if rule.status != 200:
            raise HTTPException(status_code=rule.status, detail="injected")
        return {"id": item_id, "name": f"item-{item_id}"}

    @staticmethod
    async def _sleep_unless_abandoned(seconds: float, request: Request) -> bool:
        loop = asyncio.get_running_loop()
        end = loop.time() + seconds
        while loop.time() < end:
            if await request.is_disconnected():
                return True
            await asyncio.sleep(0.01)
        return False


def unused_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


@contextmanager
def serve(app: FastAPI) -> Iterator[str]:
    """Run `app` on a free loopback port in a thread and yield its base URL."""
    server = uvicorn.Server(uvicorn.Config(app, host="127.0.0.1", port=0, log_level="warning"))
    thread = threading.Thread(target=server.run, daemon=True)
    thread.start()
    while not server.started:
        if not thread.is_alive():
            raise RuntimeError("server failed to start")
        threading.Event().wait(0.005)
    port = server.servers[0].sockets[0].getsockname()[1]
    try:
        yield f"http://127.0.0.1:{port}"
    finally:
        server.should_exit = True
        thread.join(timeout=5)
