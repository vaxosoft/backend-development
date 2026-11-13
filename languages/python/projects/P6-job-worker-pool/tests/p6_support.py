"""Test helpers: run the real app (with its lifespan) and talk to it over httpx."""

import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass, replace
from datetime import timedelta
from typing import Any

import httpx
from p6_job_worker_pool.app import create_app
from p6_job_worker_pool.config import Config

FAST = Config(
    workers=2,
    queue_capacity=8,
    max_attempts=3,
    retry_delay=timedelta(milliseconds=1),
    job_timeout=timedelta(seconds=5),
    drain_timeout=timedelta(milliseconds=100),
)


@dataclass(slots=True)
class Server:
    """The app plus its lifespan, driven by hand so tests can look at the state after shutdown."""

    client: httpx.AsyncClient
    lifespan: Any

    @classmethod
    async def start(cls, **overrides: Any) -> Server:
        app = create_app(replace(FAST, **overrides))
        lifespan = app.router.lifespan_context(app)
        await lifespan.__aenter__()
        client = httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url="http://test")
        return cls(client, lifespan)

    async def stop(self) -> None:
        await self.lifespan.__aexit__(None, None, None)

    async def aclose(self) -> None:
        await self.client.aclose()

    async def submit(self, job_type: str, payload: dict[str, Any] | None = None) -> str:
        response = await self.client.post(
            "/jobs", json={"type": job_type, "payload": payload or {}}
        )
        assert response.status_code == 202, response.text
        return str(response.json()["id"])

    async def job(self, job_id: str) -> dict[str, Any]:
        response = await self.client.get(f"/jobs/{job_id}")
        assert response.status_code == 200, response.text
        body: dict[str, Any] = response.json()
        return body

    async def wait_for(self, job_id: str, *statuses: str, timeout: float = 3.0) -> dict[str, Any]:
        """Poll until the job has one of `statuses`; fail with the last view on timeout."""
        async with asyncio.timeout(timeout):
            while True:
                view = await self.job(job_id)
                if view["status"] in statuses:
                    return view
                await asyncio.sleep(0.002)

    async def count_status(self, status: str) -> int:
        response = await self.client.get("/jobs", params={"limit": 100})
        return sum(1 for item in response.json()["items"] if item["status"] == status)


@asynccontextmanager
async def run_app(**overrides: Any) -> AsyncIterator[Server]:
    server = await Server.start(**overrides)
    try:
        yield server
    finally:
        await server.stop()
        await server.aclose()
