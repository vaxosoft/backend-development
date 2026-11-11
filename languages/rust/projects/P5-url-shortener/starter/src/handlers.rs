//! Route handlers.

use axum::Json;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::Response;

use crate::AppState;
use crate::auth::AuthUser;
use crate::error::{ApiError, ApiJson};
use crate::models::{Credentials, LinkList, LinkView, NewLink, TokenResponse, UserView};

/// TODO: `POST /auth/register` -> 201 `{"id","email"}`.
/// Email is trimmed and lowercased and must contain `@`; the password needs 8+ characters
/// (422 with `fields`). Hash on `spawn_blocking`: Argon2 would stall the async worker.
pub async fn register(
    State(_state): State<AppState>,
    ApiJson(_body): ApiJson<Credentials>,
) -> Result<(StatusCode, Json<UserView>), ApiError> {
    todo!("register")
}

/// TODO: `POST /auth/login` -> 200 `{"token"}`. Unknown email and wrong password must be
/// indistinguishable: same status, same message.
pub async fn login(
    State(_state): State<AppState>,
    ApiJson(_body): ApiJson<Credentials>,
) -> Result<Json<TokenResponse>, ApiError> {
    todo!("login")
}

/// TODO: `POST /links` -> 201 `{"slug","url"}`.
/// Order: authenticate (extractor) -> rate limit (429 + `Retry-After`) -> validate -> insert.
/// `url` must be absolute http(s); `slug` is optional, 3-32 of `[A-Za-z0-9_-]`, and not `links`;
/// without a slug generate 7 random `[A-Za-z0-9]` characters.
pub async fn create_link(
    State(_state): State<AppState>,
    _user: AuthUser,
    ApiJson(_body): ApiJson<NewLink>,
) -> Result<(StatusCode, Json<LinkView>), ApiError> {
    todo!("create a link")
}

/// TODO: `GET /{slug}` -> 302 with `Location`; unknown slug is 404. Counts a hit.
pub async fn redirect(
    State(_state): State<AppState>,
    Path(_slug): Path<String>,
) -> Result<Response, ApiError> {
    todo!("redirect")
}

/// TODO: `GET /links` -> `{"items":[{"slug","url","hits"}]}` for the caller's links only.
pub async fn list_links(
    State(_state): State<AppState>,
    _user: AuthUser,
) -> Result<Json<LinkList>, ApiError> {
    todo!("list links")
}

/// TODO: `DELETE /links/{slug}` -> 204. Unknown slug 404; somebody else's link 403.
pub async fn delete_link(
    State(_state): State<AppState>,
    _user: AuthUser,
    Path(_slug): Path<String>,
) -> Result<StatusCode, ApiError> {
    todo!("delete a link")
}
