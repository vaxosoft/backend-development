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
use axum::http::StatusCode;
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
        todo!("map each variant to its status code")
    }

    pub fn code(&self) -> &'static str {
        todo!("map each variant to its machine readable code")
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        todo!("build the envelope; add Retry-After for RateLimited; hide Internal detail")
    }
}

impl From<JsonRejection> for ApiError {
    fn from(_rejection: JsonRejection) -> Self {
        todo!(
            "JsonSyntaxError -> BadRequest, JsonDataError -> Validation{{body}}, anything else -> BadRequest"
        )
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
