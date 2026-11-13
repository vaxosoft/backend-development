"""Exercise 3: JWT authentication and role checks as FastAPI dependencies.

Implement the two TODO functions; `create_app`, `issue_token` and the error handler are given.

* `current_principal`: read the bearer token (`bearer` is already wired in), verify it with
  the app's secret (`request.app.state.jwt_secret`) and return a `Principal(user_id, role)`.
  Pin the algorithm (`algorithms=["HS256"]`), require `exp` and `sub`, and accept only
  tokens whose `sub` is an integer string and whose `role` claim is a string. Anything else
  (no header, wrong scheme, bad signature, expired, `alg: none`, missing claims) raises
  `AuthError(401, "unauthorized", ...)`. Never leak the reason for a cryptographic failure.
* `require_role(*roles)`: returns a dependency that needs a valid principal (401 otherwise)
  and raises `AuthError(403, "forbidden", ...)` unless `principal.role` is one of `roles`.
"""

from collections.abc import Callable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta
from typing import Annotated

import jwt
from fastapi import Depends, FastAPI, Request
from fastapi.responses import JSONResponse
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

bearer = HTTPBearer(auto_error=False)


@dataclass(frozen=True, slots=True)
class Principal:
    user_id: int
    role: str


class AuthError(Exception):
    def __init__(self, status: int, code: str, message: str) -> None:
        super().__init__(message)
        self.status, self.code, self.message = status, code, message


def issue_token(user_id: int, role: str, secret: str, ttl: timedelta = timedelta(hours=1)) -> str:
    now = datetime.now(UTC)
    claims = {"sub": str(user_id), "role": role, "iat": now, "exp": now + ttl}
    return jwt.encode(claims, secret, algorithm="HS256")


def _unauthorized(message: str = "invalid or missing token") -> AuthError:
    return AuthError(401, "unauthorized", message)


def current_principal(
    request: Request, creds: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)]
) -> Principal:
    if creds is None:
        raise _unauthorized("authentication required")
    try:
        claims = jwt.decode(
            creds.credentials,
            request.app.state.jwt_secret,
            algorithms=["HS256"],
            options={"require": ["exp", "sub"]},
        )
        role = claims.get("role")
        if not isinstance(role, str):
            raise _unauthorized()
        return Principal(int(claims["sub"]), role)
    except jwt.InvalidTokenError, ValueError, TypeError:
        raise _unauthorized() from None


def require_role(*roles: str) -> Callable[..., Principal]:
    def check(principal: Annotated[Principal, Depends(current_principal)]) -> Principal:
        if principal.role not in roles:
            raise AuthError(403, "forbidden", "insufficient role")
        return principal

    return check


def create_app(secret: str) -> FastAPI:
    app = FastAPI()
    app.state.jwt_secret = secret

    @app.exception_handler(AuthError)
    def auth_error(_: Request, exc: AuthError) -> JSONResponse:
        headers = {"WWW-Authenticate": "Bearer"} if exc.status == 401 else None
        body = {"error": {"code": exc.code, "message": exc.message}}
        return JSONResponse(body, status_code=exc.status, headers=headers)

    @app.get("/me")
    def me(principal: Annotated[Principal, Depends(current_principal)]) -> dict[str, object]:
        return {"user_id": principal.user_id, "role": principal.role}

    @app.get("/admin/ping")
    def admin_ping(_: Annotated[Principal, Depends(require_role("admin"))]) -> dict[str, str]:
        return {"pong": "admin"}

    @app.delete("/tasks/{task_id}", status_code=204)
    def delete_task(
        task_id: int, _: Annotated[Principal, Depends(require_role("admin", "maintainer"))]
    ) -> None:
        return None

    return app
