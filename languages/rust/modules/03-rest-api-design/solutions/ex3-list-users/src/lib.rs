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

pub fn list_users(users: &[User], query: &UserQuery) -> Result<Page<User>, QueryError> {
    let (by_name, descending) = match query.sort.as_deref().unwrap_or("id") {
        "id" => (false, false),
        "-id" => (false, true),
        "name" => (true, false),
        "-name" => (true, true),
        other => return Err(QueryError::BadSort(other.to_string())),
    };

    let needle = query.q.as_deref().unwrap_or("").to_lowercase();
    let mut matches: Vec<&User> = users
        .iter()
        .filter(|u| {
            needle.is_empty()
                || u.name.to_lowercase().contains(&needle)
                || u.email.to_lowercase().contains(&needle)
        })
        .collect();

    matches.sort_by(|a, b| {
        let primary = if by_name {
            a.name.to_lowercase().cmp(&b.name.to_lowercase())
        } else {
            a.id.cmp(&b.id)
        };
        let primary = if descending {
            primary.reverse()
        } else {
            primary
        };
        primary.then(a.id.cmp(&b.id))
    });

    let limit = query.limit.unwrap_or(DEFAULT_LIMIT).clamp(1, MAX_LIMIT);
    let offset = query.offset.unwrap_or(0);
    let total = matches.len();
    let items = matches
        .into_iter()
        .skip(offset)
        .take(limit)
        .cloned()
        .collect();
    Ok(Page {
        items,
        total,
        limit,
        offset,
    })
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
