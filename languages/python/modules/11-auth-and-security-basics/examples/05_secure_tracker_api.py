"""Task Tracker API with registration, JWT login, roles, API keys and a per-user rate limit.

Run:  python modules/11-auth-and-security-basics/examples/05_secure_tracker_api.py
Env:  ADDR (default 127.0.0.1:8080), JWT_SECRET (random per start if unset),
      ADMIN_PASSWORD (seeds admin@example.com when set)
"""

import itertools
import math
import os
import secrets
from collections.abc import Callable, Iterator
from dataclasses import dataclass, field
from typing import Annotated

import jwt
import uvicorn
from fastapi import Depends, FastAPI, Request, Response
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from fastapi.security import APIKeyHeader, HTTPAuthorizationCredentials, HTTPBearer
from m11_security import (
    RateLimiter,
    decode_token,
    hash_api_key,
    hash_password,
    issue_token,
    new_api_key,
    verify_password,
)
from pydantic import BaseModel, Field


class ApiError(Exception):
    def __init__(
        self, status: int, code: str, message: str, headers: dict[str, str] | None = None
    ) -> None:
        self.status, self.code, self.message, self.headers = status, code, message, headers


def unauthorized(message: str = "authentication required") -> ApiError:
    return ApiError(401, "unauthorized", message, {"WWW-Authenticate": "Bearer"})


class Credentials(BaseModel):
    email: str = Field(pattern=r"^[^@\s]+@[^@\s]+$", max_length=254)
    password: str = Field(min_length=8, max_length=128)


class TaskIn(BaseModel):
    title: str = Field(min_length=1, max_length=200)


@dataclass(slots=True)
class User:
    id: int
    email: str
    password_hash: str
    role: str = "user"


@dataclass
class State:
    secret: str
    limiter: RateLimiter
    users: dict[int, User] = field(default_factory=dict)
    api_keys: dict[str, int] = field(default_factory=dict)  # sha256(key) -> user id
    tasks: dict[int, str] = field(default_factory=dict)
    user_ids: Iterator[int] = field(default_factory=lambda: itertools.count(1))
    task_ids: Iterator[int] = field(default_factory=lambda: itertools.count(1))

    def by_email(self, email: str) -> User | None:
        return next((u for u in self.users.values() if u.email == email), None)


bearer = HTTPBearer(auto_error=False)
api_key_header = APIKeyHeader(name="X-API-Key", auto_error=False)
DUMMY_HASH = hash_password("not-a-real-password")  # keeps unknown-user logins as slow as real ones


def create_app(
    secret: str, admin_password: str | None = None, limiter: RateLimiter | None = None
) -> FastAPI:
    state = State(secret, limiter or RateLimiter(burst=3, refill_per_second=0.5))
    if admin_password:
        admin = User(
            next(state.user_ids), "admin@example.com", hash_password(admin_password), "admin"
        )
        state.users[admin.id] = admin
    app = FastAPI()

    @app.exception_handler(ApiError)
    def api_error(_: Request, exc: ApiError) -> JSONResponse:
        body = {"error": {"code": exc.code, "message": exc.message}}
        return JSONResponse(body, status_code=exc.status, headers=exc.headers)

    @app.exception_handler(RequestValidationError)
    def invalid(_: Request, exc: RequestValidationError) -> JSONResponse:
        fields = {".".join(str(p) for p in e["loc"][1:]): e["msg"] for e in exc.errors()}
        body = {
            "error": {"code": "validation_failed", "message": "invalid input", "fields": fields}
        }
        return JSONResponse(body, status_code=422)

    def current_user(
        creds: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)],
    ) -> User:
        if creds is None:
            raise unauthorized()
        try:
            claims = decode_token(creds.credentials, state.secret)
        except jwt.InvalidTokenError:
            raise unauthorized("invalid or expired token") from None
        user = state.users.get(int(claims["sub"]))
        if user is None:
            raise unauthorized("invalid or expired token")
        return user

    def require_role(role: str) -> Callable[[User], User]:
        def check(user: Annotated[User, Depends(current_user)]) -> User:
            if user.role != role:
                raise ApiError(403, "forbidden", f"requires role {role}")
            return user

        return check

    @app.post("/auth/register", status_code=201)
    def register(body: Credentials) -> dict[str, object]:
        if state.by_email(body.email):
            raise ApiError(409, "conflict", "email already registered")
        user = User(next(state.user_ids), body.email, hash_password(body.password))
        state.users[user.id] = user
        return {"id": user.id, "email": user.email}

    @app.post("/auth/login")
    def login(body: Credentials) -> dict[str, str]:
        user = state.by_email(body.email)
        stored = user.password_hash if user else DUMMY_HASH
        if not verify_password(body.password, stored) or user is None:
            raise unauthorized("invalid email or password")  # same message for both causes
        return {"token": issue_token(user.id, user.role, state.secret)}

    @app.get("/me")
    def me(user: Annotated[User, Depends(current_user)]) -> dict[str, object]:
        return {"id": user.id, "email": user.email, "role": user.role}

    @app.post("/tasks", status_code=201)
    def create_task(
        body: TaskIn, user: Annotated[User, Depends(current_user)]
    ) -> dict[str, object]:
        wait = state.limiter.check(str(user.id))
        if wait:
            retry = str(math.ceil(wait))
            raise ApiError(429, "rate_limited", "too many requests", {"Retry-After": retry})
        task_id = next(state.task_ids)
        state.tasks[task_id] = body.title
        return {"id": task_id, "title": body.title, "owner_id": user.id}

    @app.delete("/tasks/{task_id}", status_code=204)
    def delete_task(task_id: int, _: Annotated[User, Depends(require_role("admin"))]) -> Response:
        if state.tasks.pop(task_id, None) is None:
            raise ApiError(404, "not_found", "task not found")
        return Response(status_code=204)

    @app.post("/api-keys", status_code=201)
    def create_api_key(user: Annotated[User, Depends(current_user)]) -> dict[str, str]:
        key, digest = new_api_key()
        state.api_keys[digest] = user.id
        return {"api_key": key}  # the only time the plain key is ever visible

    @app.get("/service/ping")
    def ping(key: Annotated[str | None, Depends(api_key_header)]) -> dict[str, int]:
        owner = state.api_keys.get(hash_api_key(key)) if key else None
        if owner is None:
            raise ApiError(401, "unauthorized", "valid X-API-Key required")
        return {"owner_id": owner}

    return app


def main() -> None:
    secret = os.environ.get("JWT_SECRET") or secrets.token_urlsafe(32)
    host, _, port = os.environ.get("ADDR", "127.0.0.1:8080").rpartition(":")
    app = create_app(secret, os.environ.get("ADMIN_PASSWORD"))
    uvicorn.run(app, host=host, port=int(port))


if __name__ == "__main__":
    main()
