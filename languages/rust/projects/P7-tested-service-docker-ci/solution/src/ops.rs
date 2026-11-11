//! Readiness, metrics and the request counter behind them.

use axum::extract::{Request, State};
use axum::http::{StatusCode, header};
use axum::middleware::Next;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use serde_json::json;

use crate::AppState;

/// Routes added to the app:
///
/// * `GET /readyz`: `200 {"status":"ready"}` once `state.is_ready()`, otherwise
///   `503 {"status":"not_ready"}`
/// * `GET /metrics`: `200`, `Content-Type: text/plain; charset=utf-8`, body
///   `requests_total <n>\n` where `n` is `state.requests_total()`
pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/readyz", get(readyz))
        .route("/metrics", get(metrics))
}

async fn readyz(State(state): State<AppState>) -> impl IntoResponse {
    if state.is_ready() {
        (StatusCode::OK, Json(json!({ "status": "ready" })))
    } else {
        (
            StatusCode::SERVICE_UNAVAILABLE,
            Json(json!({ "status": "not_ready" })),
        )
    }
}

async fn metrics(State(state): State<AppState>) -> impl IntoResponse {
    (
        [(header::CONTENT_TYPE, "text/plain; charset=utf-8")],
        format!("requests_total {}\n", state.requests_total()),
    )
}

/// Middleware run for **every** request before its handler: count it with
/// `state.count_request()`. A `GET /metrics` therefore includes itself.
pub async fn count_requests(
    State(state): State<AppState>,
    request: Request,
    next: Next,
) -> Response {
    state.count_request();
    next.run(request).await
}
