"""Exercise 1: bind path, query and header parameters from type hints."""

from typing import Annotated

from fastapi import FastAPI, Header, Path, Query
from fastapi.responses import PlainTextResponse


def create_app() -> FastAPI:
    """Build the app. Routes to add (see the tests for exact JSON shapes):

    GET /hello                      -> text/plain "Hello, world!"
    GET /hello/{name}               -> text/plain "Hello, {name}!"
    GET /add?a=1&b=2                -> {"sum": 3}            (a and b are required ints)
    GET /users/{user_id}/tasks/{task_id}
                                    -> {"user_id": 3, "task_id": 7}   (task_id >= 1)
    GET /search?tag=x&tag=y&limit=5 -> {"tags": ["x", "y"], "limit": 5}
                                       (tag repeatable, optional; limit 1-50, default 10)
    GET /agent                      -> {"user_agent": "<User-Agent header>"}
    Invalid parameters must produce FastAPI's default 422.
    """
    app = FastAPI()

    @app.get("/hello", response_class=PlainTextResponse)
    @app.get("/hello/{name}", response_class=PlainTextResponse)
    def hello(name: str = "world") -> str:
        return f"Hello, {name}!"

    @app.get("/add")
    def add(a: int, b: int) -> dict[str, int]:
        return {"sum": a + b}

    @app.get("/users/{user_id}/tasks/{task_id}")
    def user_task(user_id: int, task_id: Annotated[int, Path(ge=1)]) -> dict[str, int]:
        return {"user_id": user_id, "task_id": task_id}

    @app.get("/search")
    def search(
        tag: Annotated[list[str] | None, Query()] = None,
        limit: Annotated[int, Query(ge=1, le=50)] = 10,
    ) -> dict[str, object]:
        return {"tags": tag or [], "limit": limit}

    @app.get("/agent")
    def agent(user_agent: Annotated[str | None, Header()] = None) -> dict[str, str | None]:
        return {"user_agent": user_agent}

    return app
