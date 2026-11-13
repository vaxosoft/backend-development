"""FastAPI routing: path, query and header parameters are bound from type hints.

Run:  python 02_fastapi_routing.py   (ADDR=host:port to change the address)
"""

import time
from typing import Annotated

from fastapi import FastAPI, Header, Path, Query
from fastapi.concurrency import run_in_threadpool
from fastapi.responses import PlainTextResponse
from m02_helpers import serve
from pydantic import BaseModel

app = FastAPI(title="Routing demo")


class Echo(BaseModel):
    message: str
    times: int = 1


@app.get("/hello", response_class=PlainTextResponse)
@app.get("/hello/{name}", response_class=PlainTextResponse)
def hello(name: str = "world") -> str:
    return f"Hello, {name}!"


@app.get("/users/{user_id}/tasks/{task_id}")
def task_of_user(user_id: int, task_id: Annotated[int, Path(ge=1)]) -> dict[str, int]:
    return {"user_id": user_id, "task_id": task_id}


@app.get("/search")
def search(
    q: str,
    tag: Annotated[list[str] | None, Query()] = None,
    limit: Annotated[int, Query(ge=1, le=50)] = 10,
    user_agent: Annotated[str | None, Header()] = None,
) -> dict[str, object]:
    return {"q": q, "tags": tag or [], "limit": limit, "user_agent": user_agent}


@app.post("/echo")
def echo(body: Echo) -> dict[str, object]:
    return {"echo": " ".join([body.message] * body.times)}


# A plain `def` handler runs in a worker thread, so blocking calls do not stall the event loop.
@app.get("/blocking-ok")
def blocking_ok() -> dict[str, str]:
    time.sleep(0.01)
    return {"handler": "def (threadpool)"}


# An `async def` handler runs ON the event loop: never block inside it. Offload instead.
@app.get("/blocking-offloaded")
async def blocking_offloaded() -> dict[str, str]:
    await run_in_threadpool(time.sleep, 0.01)
    return {"handler": "async def + run_in_threadpool"}


if __name__ == "__main__":
    serve(app)
