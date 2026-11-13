import asyncio
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse

from .aggregator import BadRequestError, Failure, Outcome, Success, aggregate, parse_ids
from .config import Config

CLIENT_CLOSED_REQUEST = 499
DISCONNECT_POLL_SECONDS = 0.05


def render(outcomes: list[Outcome]) -> dict[str, Any]:
    return {
        "results": [{"id": o.id, "data": o.data} for o in outcomes if isinstance(o, Success)],
        "errors": [{"id": o.id, "error": o.error} for o in outcomes if isinstance(o, Failure)],
    }


async def _wait_for_disconnect(request: Request) -> None:
    while not await request.is_disconnected():
        await asyncio.sleep(DISCONNECT_POLL_SECONDS)


def create_app(config: Config) -> FastAPI:
    @asynccontextmanager
    async def lifespan(app: FastAPI) -> AsyncIterator[None]:
        # one pooled client for the whole process, closed on shutdown
        async with httpx.AsyncClient(base_url=config.upstream_url) as client:
            app.state.client = client
            yield

    app = FastAPI(title="API aggregator", lifespan=lifespan)

    @app.exception_handler(BadRequestError)
    async def bad_request(_request: Request, exc: BadRequestError) -> JSONResponse:
        body = {"error": {"code": "bad_request", "message": str(exc)}}
        return JSONResponse(body, status_code=400)

    @app.get("/aggregate")
    async def get_aggregate(request: Request, ids: str | None = None) -> Response:
        id_list = parse_ids(ids)
        work = asyncio.create_task(aggregate(request.app.state.client, config, id_list))
        gone = asyncio.create_task(_wait_for_disconnect(request))
        try:
            await asyncio.wait({work, gone}, return_when=asyncio.FIRST_COMPLETED)
            if not work.done():
                return Response(status_code=CLIENT_CLOSED_REQUEST)
            return JSONResponse(render(work.result()))
        finally:
            # cancelling `work` propagates into every in-flight upstream call
            for task in (work, gone):
                task.cancel()
            await asyncio.gather(work, gone, return_exceptions=True)

    return app
