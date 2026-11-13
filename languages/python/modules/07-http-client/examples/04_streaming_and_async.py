"""Streaming a response body, and many concurrent requests with AsyncClient."""

import asyncio
import hashlib
import threading
import time

import httpx
from m07_helpers import Reply, Request, json_reply, serve

BIG = b"x" * (1024 * 1024)


class InFlight:
    """Counts concurrent requests inside the server (thread-safe)."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self.current = 0
        self.peak = 0

    def enter(self) -> None:
        with self._lock:
            self.current += 1
            self.peak = max(self.peak, self.current)

    def leave(self) -> None:
        with self._lock:
            self.current -= 1


in_flight = InFlight()


def upstream(request: Request) -> Reply:
    if request.path == "/lines":
        lines = "\n".join(f'{{"n": {n}}}' for n in range(5))
        return Reply(200, lines.encode(), {"Content-Type": "application/x-ndjson"})
    if request.path == "/big":
        return Reply(200, BIG, {"Content-Type": "application/octet-stream"})
    in_flight.enter()
    try:
        time.sleep(0.05)
        return json_reply({"id": int(request.path.rsplit("/", 1)[1])})
    finally:
        in_flight.leave()


def stream_demo(base_url: str) -> None:
    with httpx.Client(base_url=base_url) as client:
        # stream() does not load the body; iterate it and keep memory flat
        with client.stream("GET", "/lines") as response:
            response.raise_for_status()
            print(list(response.iter_lines()))

        digest, size = hashlib.sha256(), 0
        with client.stream("GET", "/big") as response:
            for chunk in response.iter_bytes(chunk_size=64 * 1024):
                digest.update(chunk)
                size += len(chunk)
        print(
            "downloaded",
            size,
            "bytes, sha256 matches:",
            digest.digest() == hashlib.sha256(BIG).digest(),
        )


async def async_demo(base_url: str) -> None:
    limit = asyncio.Semaphore(2)

    async def fetch(client: httpx.AsyncClient, item_id: int) -> int:
        async with limit:
            response = await client.get(f"/items/{item_id}")
            response.raise_for_status()
            return int(response.json()["id"])

    async with (
        httpx.AsyncClient(base_url=base_url, timeout=5.0) as client,
        asyncio.TaskGroup() as group,
    ):
        tasks = [group.create_task(fetch(client, n)) for n in range(1, 7)]
    print("results in request order:", [task.result() for task in tasks])
    print("peak concurrent requests at the server:", in_flight.peak)


def main() -> None:
    with serve(upstream) as base_url:
        stream_demo(base_url)
        asyncio.run(async_demo(base_url))


if __name__ == "__main__":
    main()
