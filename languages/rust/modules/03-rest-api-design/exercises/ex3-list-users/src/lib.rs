//! Exercise 3: search, sort and paginate a collection (`GET /users`).
//!
//! | Parameter | Rule |
//! |-----------|------|
//! | `q` | case-insensitive substring of the name **or** the email; empty means no filter |
//! | `sort` | `id` (default), `-id`, `name`, `-name`; names compare case-insensitively; ties by ascending id; anything else is `QueryError::BadSort` |
//! | `limit` | default 20, clamped to 1..=100 |
//! | `offset` | default 0; past the end gives no items but a correct `total` |
//!
//! `total` is the number of matches *before* slicing.

use std::sync::Arc;

use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::routing::get;
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

pub const DEFAULT_LIMIT: usize = 20;
pub const MAX_LIMIT: usize = 100;

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct User {
    pub id: u32,
    pub name: String,
    pub email: String,
}

#[derive(Debug, Default, Deserialize)]
pub struct UserQuery {
    pub q: Option<String>,
    pub sort: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
}

#[derive(Debug, PartialEq, Eq, Serialize)]
pub struct Page<T> {
    pub items: Vec<T>,
    pub total: usize,
    pub limit: usize,
    pub offset: usize,
}

#[derive(Debug, PartialEq, Eq)]
pub enum QueryError {
    BadSort(String),
}

pub fn list_users(_users: &[User], _query: &UserQuery) -> Result<Page<User>, QueryError> {
    todo!("filter with q, sort, then skip/take with the clamped limit")
}

async fn list(
    State(users): State<Arc<Vec<User>>>,
    Query(query): Query<UserQuery>,
) -> Result<Json<Page<User>>, (StatusCode, Json<serde_json::Value>)> {
    list_users(&users, &query)
        .map(Json)
        .map_err(|QueryError::BadSort(raw)| {
            (
                StatusCode::BAD_REQUEST,
                Json(serde_json::json!({ "error": {
                "code": "bad_request",
                "message": format!("unknown sort '{raw}'"),
            } })),
            )
        })
}

pub fn router(users: Vec<User>) -> Router {
    Router::new()
        .route("/users", get(list))
        .with_state(Arc::new(users))
}
