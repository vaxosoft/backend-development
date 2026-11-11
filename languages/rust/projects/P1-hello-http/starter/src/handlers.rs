//! Route handlers. Every handler is a TODO stub.

use axum::extract::rejection::QueryRejection;
use axum::extract::{Path, Query};
use axum::http::StatusCode;
use axum::response::Response;
use serde::Deserialize;

use crate::error::error_response;

/// `/slow` never sleeps longer than this.
pub const MAX_SLOW_MS: u64 = 2000;

#[derive(Debug, Deserialize)]
pub struct SlowParams {
    #[serde(default)]
    pub ms: u64,
}

fn todo_response(what: &str) -> Response {
    error_response(StatusCode::NOT_IMPLEMENTED, "todo", what)
}

/// TODO: `GET /healthz` -> 200 `{"status":"ok"}`.
pub async fn healthz() -> Response {
    todo_response("healthz")
}

/// TODO: `GET /hello` -> 200 text/plain `Hello, world!`.
pub async fn hello_world() -> Response {
    todo_response("hello_world")
}

/// TODO: `GET /hello/{name}` -> 200 text/plain `Hello, {name}!`.
pub async fn hello(Path(_name): Path<String>) -> Response {
    todo_response("hello")
}

/// TODO: `GET /slow?ms=N` -> sleep min(N, MAX_SLOW_MS) ms, then 200 `done`.
/// A non-numeric `ms` must produce a 400 `bad_request` envelope.
pub async fn slow(_params: Result<Query<SlowParams>, QueryRejection>) -> Response {
    todo_response("slow")
}

/// `GET /panic` always panics. Surviving it is the middleware's job.
pub async fn boom() -> Response {
    panic!("boom: this message must never reach the client")
}
