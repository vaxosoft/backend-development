//! Request logging middleware.

use std::time::Instant;

use axum::extract::{Request, State};
use axum::http::HeaderValue;
use axum::middleware::Next;
use axum::response::Response;
use serde_json::json;
use uuid::Uuid;

use crate::AppState;
use crate::auth::bearer_user;

const REQUEST_ID: &str = "x-request-id";

/// One JSON line per request. Only metadata is logged: the headers and the body hold
/// the password and the token, and the query string may hold secrets too.
pub async fn log_requests(State(state): State<AppState>, request: Request, next: Next) -> Response {
    let started = Instant::now();
    let method = request.method().to_string();
    let path = request.uri().path().to_string();
    let request_id = request
        .headers()
        .get(REQUEST_ID)
        .and_then(|v| v.to_str().ok())
        .filter(|v| !v.is_empty() && v.len() <= 128)
        .map_or_else(|| Uuid::new_v4().to_string(), str::to_owned);
    let user_id = bearer_user(request.headers(), &state.config.jwt_secret);

    let mut response = next.run(request).await;

    if let Ok(value) = HeaderValue::from_str(&request_id) {
        response.headers_mut().insert(REQUEST_ID, value);
    }
    let mut line = json!({
        "request_id": request_id,
        "method": method,
        "path": path,
        "status": response.status().as_u16(),
        "duration_ms": started.elapsed().as_millis() as u64,
    });
    if let Some(id) = user_id {
        line["user_id"] = json!(id);
    }
    (state.config.log_sink)(line.to_string());
    response
}
