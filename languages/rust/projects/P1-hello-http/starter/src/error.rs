//! The JSON error envelope shared by every failure response.

use axum::Json;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::json;

/// Builds `{"error":{"code":...,"message":...}}` with the given status.
pub fn error_response(status: StatusCode, code: &str, message: &str) -> Response {
    let body = json!({"error": {"code": code, "message": message}});
    (status, Json(body)).into_response()
}
