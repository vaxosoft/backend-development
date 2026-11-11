//! Route handlers.

use std::time::Duration;

use axum::Json;
use axum::extract::rejection::QueryRejection;
use axum::extract::{Path, Query};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde::Deserialize;
use serde_json::{Value, json};

use crate::error::error_response;

/// `/slow` never sleeps longer than this.
pub const MAX_SLOW_MS: u64 = 2000;

#[derive(Debug, Deserialize)]
pub struct SlowParams {
    #[serde(default)]
    pub ms: u64,
}

pub async fn healthz() -> Json<Value> {
    Json(json!({"status": "ok"}))
}

pub async fn hello_world() -> &'static str {
    "Hello, world!"
}

pub async fn hello(Path(name): Path<String>) -> String {
    format!("Hello, {name}!")
}

// The rejection is mapped by hand: the extractor's default error is plain text.
pub async fn slow(params: Result<Query<SlowParams>, QueryRejection>) -> Response {
    let Ok(Query(params)) = params else {
        return error_response(
            StatusCode::BAD_REQUEST,
            "bad_request",
            "ms must be a non-negative integer",
        );
    };
    tokio::time::sleep(Duration::from_millis(params.ms.min(MAX_SLOW_MS))).await;
    "done".into_response()
}

/// `GET /panic` always panics. Surviving it is the middleware's job.
pub async fn boom() -> Response {
    panic!("boom: this message must never reach the client")
}
