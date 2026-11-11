//! One JSON error envelope for every failure, plus extractors that use it.
//!
//! ```json
//! {"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
//! ```

use std::collections::BTreeMap;

use axum::Json;
use axum::extract::rejection::{JsonRejection, PathRejection, QueryRejection};
use axum::extract::{FromRequest, FromRequestParts};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde_json::json;

/// Field name -> human readable problem.
pub type FieldErrors = BTreeMap<String, String>;

#[derive(Debug, thiserror::Error)]
pub enum ApiError {
    #[error("{0}")]
    BadRequest(String),
    #[error("{0}")]
    NotFound(String),
    #[error("{message}")]
    Validation {
        message: String,
        fields: FieldErrors,
    },
    #[error("{0}")]
    Conflict(String),
    #[error("payload too large")]
    PayloadTooLarge,
    /// The detail is for the log only; clients get a generic message.
    #[error("internal error: {0}")]
    Internal(String),
}

impl ApiError {
    pub fn validation(message: &str, fields: FieldErrors) -> Self {
        ApiError::Validation {
            message: message.to_string(),
            fields,
        }
    }

    pub fn status(&self) -> StatusCode {
        match self {
            ApiError::BadRequest(_) => StatusCode::BAD_REQUEST,
            ApiError::NotFound(_) => StatusCode::NOT_FOUND,
            ApiError::Validation { .. } => StatusCode::UNPROCESSABLE_ENTITY,
            ApiError::Conflict(_) => StatusCode::CONFLICT,
            ApiError::PayloadTooLarge => StatusCode::PAYLOAD_TOO_LARGE,
            ApiError::Internal(_) => StatusCode::INTERNAL_SERVER_ERROR,
        }
    }

    pub fn code(&self) -> &'static str {
        match self {
            ApiError::BadRequest(_) => "bad_request",
            ApiError::NotFound(_) => "not_found",
            ApiError::Validation { .. } => "validation_failed",
            ApiError::Conflict(_) => "conflict",
            ApiError::PayloadTooLarge => "payload_too_large",
            ApiError::Internal(_) => "internal",
        }
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let message = match &self {
            ApiError::Internal(detail) => {
                eprintln!("internal error: {detail}");
                "internal server error".to_string()
            }
            other => other.to_string(),
        };
        let mut error = json!({ "code": self.code(), "message": message });
        if let ApiError::Validation { fields, .. } = &self {
            error["fields"] = json!(fields);
        }
        (self.status(), Json(json!({ "error": error }))).into_response()
    }
}

impl From<JsonRejection> for ApiError {
    fn from(rejection: JsonRejection) -> Self {
        match rejection {
            // Not JSON at all: the client sent a broken request.
            JsonRejection::JsonSyntaxError(err) => {
                ApiError::BadRequest(format!("malformed JSON: {err}"))
            }
            // Valid JSON of the wrong shape (e.g. `"title": 5`): well-formed but invalid.
            JsonRejection::JsonDataError(err) => ApiError::validation(
                "invalid request body",
                FieldErrors::from([("body".to_string(), err.body_text())]),
            ),
            other if other.status() == StatusCode::PAYLOAD_TOO_LARGE => ApiError::PayloadTooLarge,
            other => ApiError::BadRequest(other.body_text()),
        }
    }
}

impl From<QueryRejection> for ApiError {
    fn from(rejection: QueryRejection) -> Self {
        ApiError::BadRequest(rejection.body_text())
    }
}

impl From<PathRejection> for ApiError {
    fn from(rejection: PathRejection) -> Self {
        ApiError::BadRequest(rejection.body_text())
    }
}

/// `Json<T>` whose rejections use the error envelope.
#[derive(FromRequest)]
#[from_request(via(axum::Json), rejection(ApiError))]
pub struct ApiJson<T>(pub T);

/// `Query<T>` whose rejections use the error envelope.
#[derive(FromRequestParts)]
#[from_request(via(axum::extract::Query), rejection(ApiError))]
pub struct ApiQuery<T>(pub T);

/// `Path<T>` whose rejections use the error envelope.
#[derive(FromRequestParts)]
#[from_request(via(axum::extract::Path), rejection(ApiError))]
pub struct ApiPath<T>(pub T);

/// Router fallback: unknown routes get the same envelope as every other error.
pub async fn not_found_fallback() -> ApiError {
    ApiError::NotFound("no such route".to_string())
}
