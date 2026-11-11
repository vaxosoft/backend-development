//! Exercise 1: routes, path and query extractors, and a fallback.
//!
//! Routes to build in `router()`:
//!
//! | Request | Response |
//! |---------|----------|
//! | `GET /hello` | `200` text `Hello, world!` |
//! | `GET /hello/{name}` | `200` text `Hello, {name}!`; `?greeting=Hi` replaces `Hello` |
//! | `GET /add?a=2&b=3` | `200` text `5`; a missing or non-numeric value is a `400` |
//! | `GET /users/{user_id}/tasks/{task_id}` | `200` JSON `{"user_id":1,"task_id":2}` (numbers; non-numeric ids are `400`) |
//! | anything else | `404` text `not found` |

use axum::Router;
use axum::extract::{Path, Query};
use axum::http::StatusCode;
use axum::routing::get;
use serde::Deserialize;

#[derive(Debug, Deserialize)]
pub struct GreetingQuery {
    pub greeting: Option<String>,
}

#[derive(Debug, Deserialize)]
pub struct AddQuery {
    pub a: i64,
    pub b: i64,
}

async fn hello_world() -> &'static str {
    todo!("return the default greeting")
}

async fn hello(Path(_name): Path<String>, Query(_q): Query<GreetingQuery>) -> String {
    todo!("use the greeting from the query string, defaulting to Hello")
}

async fn add(Query(_q): Query<AddQuery>) -> String {
    todo!("add the two numbers")
}

async fn user_task(Path((_user_id, _task_id)): Path<(u32, u32)>) -> axum::Json<serde_json::Value> {
    todo!("return the two ids as JSON")
}

async fn not_found() -> (StatusCode, &'static str) {
    todo!("answer 404 with the body `not found`")
}

pub fn router() -> Router {
    Router::new()
        .route("/hello", get(hello_world))
        .route("/hello/{name}", get(hello))
        .route("/add", get(add))
        .route("/users/{user_id}/tasks/{task_id}", get(user_task))
        .fallback(not_found)
}
