"""Middleware order, CORS, dependencies, lifespan, static files and graceful shutdown.

Run:  python 04_middleware_and_lifespan.py   (ADDR=host:port to change the address)
"""

import asyncio
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Annotated

from fastapi import Depends, FastAPI, Header, HTTPException, Request, Response
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from m02_helpers import serve

STATIC_DIR = Path(__file__).parent / "static"


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    app.state.ready = True  # create shared resources (pools, clients) here
    print("startup", flush=True)
    yield
    print("shutdown", flush=True)  # runs after in-flight requests have drained


app = FastAPI(lifespan=lifespan)

Next = Callable[[Request], Awaitable[Response]]


def tag_middleware(name: str) -> Callable[[Request, Next], Awaitable[Response]]:
    async def middleware(request: Request, call_next: Next) -> Response:
        request.state.trail = [*getattr(request.state, "trail", []), name]
        response = await call_next(request)
        previous = response.headers.get("X-Order")
        response.headers["X-Order"] = f"{previous},{name}" if previous else name
        return response

    return middleware


# add_middleware wraps the app: the LAST one added is the OUTERMOST (first to see the request).
app.middleware("http")(tag_middleware("inner"))
app.middleware("http")(tag_middleware("outer"))
app.add_middleware(
    CORSMiddleware,
    allow_origins=["https://app.example.com"],
    allow_methods=["GET", "POST"],
    allow_headers=["Content-Type", "X-API-Key"],
    max_age=600,
)


def require_api_key(x_api_key: Annotated[str | None, Header()] = None) -> str:
    if x_api_key != "secret":
        raise HTTPException(status_code=401, detail="missing or invalid API key")
    return x_api_key


@app.get("/readyz")
def readyz(request: Request) -> dict[str, bool]:
    return {"ready": request.app.state.ready}


@app.get("/trail")
def trail(request: Request) -> dict[str, list[str]]:
    return {"request_order": request.state.trail}


@app.get("/private")
def private(key: Annotated[str, Depends(require_api_key)]) -> dict[str, str]:
    return {"key_length": str(len(key))}


@app.get("/slow")
async def slow(ms: int = 100) -> dict[str, str]:
    await asyncio.sleep(min(max(ms, 0), 2000) / 1000)
    return {"status": "done"}


app.mount("/static", StaticFiles(directory=STATIC_DIR), name="static")

if __name__ == "__main__":
    serve(app)
