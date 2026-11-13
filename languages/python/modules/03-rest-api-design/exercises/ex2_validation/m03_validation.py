"""Exercise 2: validate input with Pydantic and answer every failure with the error envelope."""

from typing import Any

from fastapi import FastAPI, Query
from pydantic import BaseModel


class TaskIn(BaseModel):
    """Make this strict. Rules:

    - title: required, stripped of surrounding whitespace, 1-200 characters
    - status: "todo" (default), "in_progress" or "done"
    - due_date: optional YYYY-MM-DD date (null allowed)
    - tags: 0-10 unique tags, each 1-30 characters and lowercase
    - unknown fields are rejected
    """

    title: str
    status: str = "todo"
    due_date: str | None = None
    tags: list[str] = []


def install_error_handlers(app: FastAPI) -> None:
    """Every error response is {"error": {"code", "message", "fields"?}}:

    - malformed JSON body            -> 400 "bad_request"
    - bad path/query/header value    -> 400 "bad_request", "fields" names the parameter
    - well-formed but invalid body   -> 422 "validation_failed", "fields" maps field -> message
                                        (nested locations joined with ".", e.g. "tags.0")
    - HTTP errors (404, 405, ...)    -> same status, codes "not_found", "method_not_allowed", ...
    - any other exception            -> 500 "internal", message "internal server error"
    """
    raise NotImplementedError


def create_app() -> FastAPI:
    app = FastAPI()
    install_error_handlers(app)

    @app.post("/tasks", status_code=201)
    def create(data: TaskIn) -> dict[str, Any]:
        return {"id": 1, **data.model_dump(mode="json")}

    @app.get("/repeat")
    def repeat(text: str, count: int = Query(ge=1, le=5)) -> dict[str, str]:
        return {"result": text * count}

    @app.get("/boom")
    def boom() -> None:
        raise RuntimeError("secret internal detail")

    return app
