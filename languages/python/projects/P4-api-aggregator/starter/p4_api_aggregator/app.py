from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

from .aggregator import BadRequestError, Failure, Outcome, Success, aggregate, parse_ids
from .config import Config


def render(outcomes: list[Outcome]) -> dict[str, Any]:
    return {
        "results": [{"id": o.id, "data": o.data} for o in outcomes if isinstance(o, Success)],
        "errors": [{"id": o.id, "error": o.error} for o in outcomes if isinstance(o, Failure)],
    }


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
    async def get_aggregate(request: Request, ids: str | None = None) -> JSONResponse:
        id_list = parse_ids(ids)
        outcomes = await aggregate(request.app.state.client, config, id_list)
        return JSONResponse(render(outcomes))

    return app
