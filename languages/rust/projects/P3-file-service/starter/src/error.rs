//! One error type for the whole API; it renders the JSON envelope.

use std::io;

use axum::Json;
use axum::extract::multipart::MultipartError;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::json;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ApiError {
    #[error("bad request: {0}")]
    BadRequest(String),
    #[error("not found")]
    NotFound,
    #[error("payload too large")]
    PayloadTooLarge,
    #[error("internal error")]
    Internal,
}

/// I/O failures are logged here and never shown to the client.
impl From<io::Error> for ApiError {
    fn from(err: io::Error) -> Self {
        eprintln!("i/o error: {err}");
        Self::Internal
    }
}

/// A body-size limit hit inside the multipart parser is a 413, anything else a 400.
impl From<MultipartError> for ApiError {
    fn from(err: MultipartError) -> Self {
        if err.status() == StatusCode::PAYLOAD_TOO_LARGE {
            Self::PayloadTooLarge
        } else {
            Self::BadRequest("malformed multipart body".to_owned())
        }
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code, message) = match self {
            Self::BadRequest(message) => (StatusCode::BAD_REQUEST, "bad_request", message),
            Self::NotFound => (
                StatusCode::NOT_FOUND,
                "not_found",
                "file not found".to_owned(),
            ),
            Self::PayloadTooLarge => (
                StatusCode::PAYLOAD_TOO_LARGE,
                "payload_too_large",
                "file is too large".to_owned(),
            ),
            Self::Internal => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "internal",
                "internal server error".to_owned(),
            ),
        };
        let body = json!({"error": {"code": code, "message": message}});
        (status, Json(body)).into_response()
    }
}
