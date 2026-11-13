import math
import secrets
import string
from datetime import UTC, datetime
from typing import Annotated, Any

from fastapi import Depends, FastAPI, Request, Response
from fastapi.responses import RedirectResponse
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from .access_log import install_access_log
from .config import Config
from .db import connect
from .errors import AppError, install_error_handlers
from .models import Link, LinkCreate, LoginRequest, RegisterRequest
from .rate_limit import RateLimiter
from .security import (
    InvalidTokenError,
    decode_token,
    hash_password,
    issue_token,
    verify_password,
)
from .store import EmailTakenError, SlugTakenError, Store

SLUG_ALPHABET = string.ascii_letters + string.digits
GENERATED_SLUG_LENGTH = 7
RESERVED_SLUGS = frozenset({"auth", "links"})  # first path segments owned by other routes
MAX_PAGE_SIZE = 100


def _unauthorized() -> AppError:
    return AppError(
        401,
        "unauthorized",
        "missing or invalid credentials",
        headers={"WWW-Authenticate": "Bearer"},
    )


def _link_json(link: Link) -> dict[str, Any]:
    return {"slug": link.slug, "url": link.url, "hits": link.hits, "created_at": link.created_at}


def create_app(config: Config) -> FastAPI:
    store = Store(connect(config.db_path))
    limiter = RateLimiter(config.rate_burst, config.rate_refill_per_second, config.clock)
    # verified when the email is unknown, so both login failures take about the same time
    dummy_hash = hash_password("not-a-real-password", config.scrypt_n)
    bearer = HTTPBearer(auto_error=False)

    app = FastAPI(title="URL shortener", docs_url=None, redoc_url=None, openapi_url=None)
    install_error_handlers(app)
    install_access_log(app, config)

    def current_user_id(
        request: Request,
        credentials: Annotated[HTTPAuthorizationCredentials | None, Depends(bearer)],
    ) -> int:
        if credentials is None:
            raise _unauthorized()
        try:
            user_id = decode_token(config, credentials.credentials)
        except InvalidTokenError as err:
            raise _unauthorized() from err
        request.state.user_id = user_id  # read by the access log middleware
        return user_id

    UserId = Annotated[int, Depends(current_user_id)]

    def limit_link_creation(user_id: UserId) -> None:
        wait = limiter.try_acquire(str(user_id))
        if wait > 0:
            retry_after = str(max(1, math.ceil(wait)))
            raise AppError(
                429, "rate_limited", "too many links created", headers={"Retry-After": retry_after}
            )

    @app.post("/auth/register", status_code=201)
    def register(body: RegisterRequest) -> dict[str, Any]:
        password_hash = hash_password(body.password, config.scrypt_n)
        try:
            user = store.create_user(body.email, password_hash)
        except EmailTakenError as err:
            raise AppError(409, "conflict", "email already registered") from err
        return {"id": user.id, "email": user.email}

    @app.post("/auth/login")
    def login(body: LoginRequest, request: Request) -> dict[str, str]:
        user = store.get_user_by_email(body.email.strip().lower())
        password_ok = verify_password(body.password, user.password_hash if user else dummy_hash)
        if user is None or not password_ok:
            raise AppError(401, "unauthorized", "invalid email or password")
        request.state.user_id = user.id
        return {"token": issue_token(config, user.id)}

    @app.post("/links", status_code=201, dependencies=[Depends(limit_link_creation)])
    def create_link(body: LinkCreate, user_id: UserId, response: Response) -> dict[str, str]:
        created_at = datetime.now(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")
        if body.slug is None:
            link = _create_with_generated_slug(store, user_id, body.url, created_at)
        elif body.slug in RESERVED_SLUGS:
            raise AppError(422, "validation_failed", "invalid request", {"slug": "is reserved"})
        else:
            try:
                link = store.create_link(user_id, body.slug, body.url, created_at)
            except SlugTakenError as err:
                raise AppError(409, "conflict", "slug already in use") from err
        response.headers["Location"] = f"/{link.slug}"
        return {"slug": link.slug, "url": link.url}

    @app.get("/links")
    def list_links(user_id: UserId, limit: int = 20, offset: int = 0) -> dict[str, Any]:
        limit = min(max(limit, 1), MAX_PAGE_SIZE)
        offset = max(offset, 0)
        links, total = store.list_links(user_id, limit, offset)
        return {
            "items": [_link_json(link) for link in links],
            "total": total,
            "limit": limit,
            "offset": offset,
        }

    @app.delete("/links/{slug}")
    def delete_link(slug: str, user_id: UserId) -> Response:
        link = store.get_link(slug)
        if link is None:
            raise AppError(404, "not_found", "unknown slug")
        if link.owner_id != user_id:
            raise AppError(403, "forbidden", "only the owner can delete a link")
        store.delete_link(slug)
        return Response(status_code=204)

    # registered last: it matches any single path segment
    @app.get("/{slug}")
    def follow(slug: str) -> RedirectResponse:
        url = store.record_hit(slug)
        if url is None:
            raise AppError(404, "not_found", "unknown slug")
        return RedirectResponse(url, status_code=302)

    return app


def _create_with_generated_slug(store: Store, owner_id: int, url: str, created_at: str) -> Link:
    for _ in range(5):
        slug = "".join(secrets.choice(SLUG_ALPHABET) for _ in range(GENERATED_SLUG_LENGTH))
        try:
            return store.create_link(owner_id, slug, url, created_at)
        except SlugTakenError:
            continue
    raise AppError(500, "internal", "could not generate a unique slug")
