//! Request logging middleware.

use axum::extract::{Request, State};
use axum::middleware::Next;
use axum::response::Response;

use crate::AppState;

/// TODO: emit exactly one JSON line per request through `state.config.log_sink`, after the
/// response is known:
/// `{"request_id","method","path","status","duration_ms"}` plus `"user_id"` (a number) when
/// the request carried a valid bearer token.
///
/// - `request_id` is the incoming `X-Request-ID` header, or a fresh UUID; the same value is
///   set on the response as `X-Request-ID`.
/// - Log `path` only (no query string) and never the headers or the body: they hold the
///   password and the token.
pub async fn log_requests(
    State(_state): State<AppState>,
    request: Request,
    next: Next,
) -> Response {
    next.run(request).await
}
