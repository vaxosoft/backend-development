//! Exercise 1: a typed `ApiError` that renders the JSON error envelope.
//!
//! ```json
//! {"error":{"code":"validation_failed","message":"invalid item","fields":{"name":"is required"}}}
//! ```
//!
//! | Variant | Status | `code` | `message` |
//! |---------|--------|--------|-----------|
//! | `BadRequest(msg)` | 400 | `bad_request` | `msg` |
//! | `Unauthorized` | 401 | `unauthorized` | `authentication required` |
//! | `Forbidden` | 403 | `forbidden` | `not allowed` |
//! | `NotFound(msg)` | 404 | `not_found` | `msg` |
//! | `Validation { message, fields }` | 422 | `validation_failed` | `message`, plus the `fields` object |
//! | `RateLimited { retry_after_secs }` | 429 | `rate_limited` | `too many requests`, plus header `Retry-After: <secs>` |
//! | `Internal(detail)` | 500 | `internal` | `internal server error` (never the detail) |
//!
//! `fields` appears only for `Validation`. Invalid JSON bodies must use the
//! envelope too: a syntax error is a `BadRequest`, JSON of the wrong shape is a
//! `Validation` with a single field named `body`.

use std::collections::BTreeMap;

use axum::extract::rejection::JsonRejection;
use axum::extract::{FromRequest, Path};
use axum::http::{StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::Deserialize;

#[derive(Debug)]
pub enum ApiError {
    BadRequest(String),
    Unauthorized,
    Forbidden,
    NotFound(String),
    Validation {
        message: String,
        fields: BTreeMap<String, String>,
    },
    RateLimited {
        retry_after_secs: u64,
    },
    Internal(String),
}

impl ApiError {
    pub fn status(&self) -> StatusCode {
        match self {
            ApiError::BadRequest(_) => StatusCode::BAD_REQUEST,
            ApiError::Unauthorized => StatusCode::UNAUTHORIZED,
            ApiError::Forbidden => StatusCode::FORBIDDEN,
            ApiError::NotFound(_) => StatusCode::NOT_FOUND,
            ApiError::Validation { .. } => StatusCode::UNPROCESSABLE_ENTITY,
            ApiError::RateLimited { .. } => StatusCode::TOO_MANY_REQUESTS,
            ApiError::Internal(_) => StatusCode::INTERNAL_SERVER_ERROR,
        }
    }

    pub fn code(&self) -> &'static str {
        match self {
            ApiError::BadRequest(_) => "bad_request",
            ApiError::Unauthorized => "unauthorized",
            ApiError::Forbidden => "forbidden",
            ApiError::NotFound(_) => "not_found",
            ApiError::Validation { .. } => "validation_failed",
            ApiError::RateLimited { .. } => "rate_limited",
            ApiError::Internal(_) => "internal",
        }
    }

    fn message(&self) -> String {
        match self {
            ApiError::BadRequest(message) | ApiError::NotFound(message) => message.clone(),
            ApiError::Unauthorized => "authentication required".to_string(),
            ApiError::Forbidden => "not allowed".to_string(),
            ApiError::Validation { message, .. } => message.clone(),
            ApiError::RateLimited { .. } => "too many requests".to_string(),
            ApiError::Internal(_) => "internal server error".to_string(),
        }
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let mut error = serde_json::json!({ "code": self.code(), "message": self.message() });
        if let ApiError::Validation { fields, .. } = &self {
            error["fields"] = serde_json::json!(fields);
        }
        let mut response =
            (self.status(), Json(serde_json::json!({ "error": error }))).into_response();
        if let ApiError::RateLimited { retry_after_secs } = self {
            response
                .headers_mut()
                .insert(header::RETRY_AFTER, retry_after_secs.into());
        }
        response
    }
}

impl From<JsonRejection> for ApiError {
    fn from(rejection: JsonRejection) -> Self {
        match rejection {
            JsonRejection::JsonSyntaxError(err) => ApiError::BadRequest(err.body_text()),
            JsonRejection::JsonDataError(err) => ApiError::Validation {
                message: "invalid request body".to_string(),
                fields: BTreeMap::from([("body".to_string(), err.body_text())]),
            },
            other => ApiError::BadRequest(other.body_text()),
        }
    }
}

/// `Json<T>` that reports rejections through `ApiError`.
#[derive(FromRequest)]
#[from_request(via(axum::Json), rejection(ApiError))]
pub struct ApiJson<T>(pub T);

#[derive(Deserialize)]
struct Item {
    name: String,
}

async fn create_item(ApiJson(item): ApiJson<Item>) -> (StatusCode, Json<serde_json::Value>) {
    (
        StatusCode::CREATED,
        Json(serde_json::json!({ "name": item.name })),
    )
}

/// `GET /fail/{kind}` returns one error per kind so every variant can be tested.
async fn fail(Path(kind): Path<String>) -> ApiError {
    match kind.as_str() {
        "bad_request" => ApiError::BadRequest("nope".into()),
        "unauthorized" => ApiError::Unauthorized,
        "forbidden" => ApiError::Forbidden,
        "not_found" => ApiError::NotFound("no such thing".into()),
        "validation" => ApiError::Validation {
            message: "invalid item".into(),
            fields: BTreeMap::from([("name".to_string(), "is required".to_string())]),
        },
        "rate_limited" => ApiError::RateLimited {
            retry_after_secs: 7,
        },
        _ => ApiError::Internal(format!("secret detail about {kind}")),
    }
}

pub fn router() -> Router {
    Router::new()
        .route("/items", post(create_item))
        .route("/fail/{kind}", get(fail))
}
