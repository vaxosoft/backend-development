//! One error type for the whole API; it knows how to render the JSON envelope.

use std::collections::BTreeMap;

use axum::Json;
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::{Value, json};
use thiserror::Error;

#[derive(Debug, Error)]
pub enum ApiError {
    /// Malformed JSON or a bad query value (400).
    #[error("bad request: {0}")]
    BadRequest(String),
    #[error("not found")]
    NotFound,
    /// Well-formed but invalid data: field name -> message (422).
    #[error("validation failed")]
    Validation(BTreeMap<String, String>),
    #[error("internal error")]
    Internal,
}

impl ApiError {
    /// Keeps only the checks that failed: `("title", Some("must be ..."))`.
    pub fn validation(
        checks: impl IntoIterator<Item = (&'static str, Option<&'static str>)>,
    ) -> Self {
        let fields = checks
            .into_iter()
            .filter_map(|(field, problem)| Some((field.to_owned(), problem?.to_owned())))
            .collect();
        Self::Validation(fields)
    }
}

/// Database failures are logged here and never shown to the client.
impl From<sqlx::Error> for ApiError {
    fn from(err: sqlx::Error) -> Self {
        eprintln!("database error: {err}");
        Self::Internal
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code, message, fields) = match self {
            Self::BadRequest(message) => (StatusCode::BAD_REQUEST, "bad_request", message, None),
            Self::NotFound => (
                StatusCode::NOT_FOUND,
                "not_found",
                "task not found".to_owned(),
                None,
            ),
            Self::Validation(fields) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "validation_failed",
                "invalid task".to_owned(),
                Some(fields),
            ),
            Self::Internal => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "internal",
                "internal server error".to_owned(),
                None,
            ),
        };
        let mut error = json!({"code": code, "message": message});
        if let Some(fields) = fields {
            error["fields"] = Value::from_iter(fields);
        }
        (status, Json(json!({ "error": error }))).into_response()
    }
}
