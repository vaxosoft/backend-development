"""Exercise 2: validate input with Pydantic and answer every failure with the error envelope."""

from datetime import date
from typing import Annotated, Any

from fastapi import FastAPI, Query, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel, ConfigDict, Field, StringConstraints, field_validator
from starlette.exceptions import HTTPException as StarletteHTTPException

STATUS_CODES = {
    400: "bad_request",
    401: "unauthorized",
    403: "forbidden",
    404: "not_found",
    405: "method_not_allowed",
    409: "conflict",
    422: "validation_failed",
}

Tag = Annotated[str, StringConstraints(min_length=1, max_length=30)]


class TaskIn(BaseModel):
    """Make this strict. Rules:

    - title: required, stripped of surrounding whitespace, 1-200 characters
    - status: "todo" (default), "in_progress" or "done"
    - due_date: optional YYYY-MM-DD date (null allowed)
    - tags: 0-10 unique tags, each 1-30 characters and lowercase
    - unknown fields are rejected
    """

    model_config = ConfigDict(extra="forbid")

    title: Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)]
    status: Annotated[str, Field(pattern=r"^(todo|in_progress|done)$")] = "todo"
    due_date: date | None = None
    tags: Annotated[list[Tag], Field(max_length=10)] = Field(default_factory=list)

    @field_validator("tags")
    @classmethod
    def lowercase_and_unique(cls, tags: list[str]) -> list[str]:
        if any(tag != tag.lower() for tag in tags):
            raise ValueError("tags must be lowercase")
        if len(set(tags)) != len(tags):
            raise ValueError("tags must be unique")
        return tags


def envelope(
    status: int, code: str, message: str, fields: dict[str, str] | None = None
) -> JSONResponse:
    body: dict[str, Any] = {"code": code, "message": message}
    if fields:
        body["fields"] = fields
    return JSONResponse({"error": body}, status_code=status)


def install_error_handlers(app: FastAPI) -> None:
    """Every error response is {"error": {"code", "message", "fields"?}}:

    - malformed JSON body            -> 400 "bad_request"
    - bad path/query/header value    -> 400 "bad_request", "fields" names the parameter
    - well-formed but invalid body   -> 422 "validation_failed", "fields" maps field -> message
                                        (nested locations joined with ".", e.g. "tags.0")
    - HTTP errors (404, 405, ...)    -> same status, codes "not_found", "method_not_allowed", ...
    - any other exception            -> 500 "internal", message "internal server error"
    """

    @app.exception_handler(RequestValidationError)
    async def on_validation(request: Request, exc: RequestValidationError) -> JSONResponse:
        errors = exc.errors()
        if any(e["type"] == "json_invalid" for e in errors):
            return envelope(400, "bad_request", "malformed JSON body")
        fields: dict[str, str] = {}
        for e in errors:
            name = ".".join(str(part) for part in e["loc"][1:]) or "body"
            fields.setdefault(name, e["msg"].removeprefix("Value error, "))
        if any(e["loc"][0] != "body" for e in errors):
            return envelope(400, "bad_request", "invalid request parameters", fields)
        return envelope(422, "validation_failed", "invalid request", fields)

    @app.exception_handler(StarletteHTTPException)
    async def on_http(request: Request, exc: StarletteHTTPException) -> JSONResponse:
        code = STATUS_CODES.get(exc.status_code, "internal" if exc.status_code >= 500 else "error")
        return envelope(exc.status_code, code, str(exc.detail))

    @app.exception_handler(Exception)
    async def on_unexpected(request: Request, exc: Exception) -> JSONResponse:
        return envelope(500, "internal", "internal server error")


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
