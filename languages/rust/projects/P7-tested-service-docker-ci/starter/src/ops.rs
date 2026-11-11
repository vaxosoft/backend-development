//! Readiness, metrics and the request counter behind them.

use axum::Router;
use axum::extract::{Request, State};
use axum::middleware::Next;
use axum::response::Response;

use crate::AppState;

/// Routes added to the app:
///
/// * `GET /readyz`: `200 {"status":"ready"}` once `state.is_ready()`, otherwise
///   `503 {"status":"not_ready"}`
/// * `GET /metrics`: `200`, `Content-Type: text/plain; charset=utf-8`, body
///   `requests_total <n>\n` where `n` is `state.requests_total()`
pub fn routes() -> Router<AppState> {
    // TODO: add the two routes and their handlers.
    Router::new()
}

/// Middleware run for **every** request before its handler: count it with
/// `state.count_request()`. A `GET /metrics` therefore includes itself.
pub async fn count_requests(
    State(_state): State<AppState>,
    request: Request,
    next: Next,
) -> Response {
    // TODO: count the request.
    next.run(request).await
}
