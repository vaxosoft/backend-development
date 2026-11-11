//! Routes, handlers and the JSON error envelope.

use std::any::Any;
use std::collections::BTreeMap;

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::{HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use serde::Serialize;
use serde_json::{Value, json};
use tower_http::catch_panic::CatchPanicLayer;

use crate::jobs::{JobSpec, JobView};
use crate::pool::{CancelError, Pool, SubmitError};

pub fn router(pool: Pool) -> Router {
    Router::new()
        .route("/jobs", get(list_jobs).post(create_job))
        .route("/jobs/{id}", get(get_job).delete(cancel_job))
        .fallback(|| async { ApiError::NotFound })
        .layer(CatchPanicLayer::custom(|_: Box<dyn Any + Send>| {
            ApiError::Internal.into_response()
        }))
        .with_state(pool)
}

#[derive(Debug)]
pub enum ApiError {
    BadRequest(String),
    Validation(BTreeMap<String, String>),
    NotFound,
    Conflict(String),
    RateLimited,
    Unavailable,
    Internal,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code, message, fields) = match self {
            ApiError::BadRequest(msg) => (StatusCode::BAD_REQUEST, "bad_request", msg, None),
            ApiError::Validation(fields) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "validation_failed",
                "invalid job".to_owned(),
                Some(fields),
            ),
            ApiError::NotFound => (StatusCode::NOT_FOUND, "not_found", "not found".into(), None),
            ApiError::Conflict(msg) => (StatusCode::CONFLICT, "conflict", msg, None),
            ApiError::RateLimited => (
                StatusCode::TOO_MANY_REQUESTS,
                "rate_limited",
                "the job queue is full, retry later".into(),
                None,
            ),
            ApiError::Unavailable => (
                StatusCode::SERVICE_UNAVAILABLE,
                "unavailable",
                "the service is shutting down".into(),
                None,
            ),
            ApiError::Internal => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "internal",
                "internal error".into(),
                None,
            ),
        };
        let mut error = json!({ "code": code, "message": message });
        if let Some(fields) = fields {
            error["fields"] = json!(fields);
        }
        let mut response = (status, Json(json!({ "error": error }))).into_response();
        if status == StatusCode::TOO_MANY_REQUESTS {
            response
                .headers_mut()
                .insert(header::RETRY_AFTER, HeaderValue::from_static("1"));
        }
        response
    }
}

// The body is taken as raw bytes so that malformed JSON (400) and well-formed
// but invalid jobs (422) can be told apart.
async fn create_job(State(pool): State<Pool>, body: Bytes) -> Result<Response, ApiError> {
    let value: Value = serde_json::from_slice(&body)
        .map_err(|e| ApiError::BadRequest(format!("malformed JSON: {e}")))?;
    let spec = JobSpec::parse(&value).map_err(ApiError::Validation)?;
    let id = pool.submit(spec).map_err(|e| match e {
        SubmitError::QueueFull => ApiError::RateLimited,
        SubmitError::ShuttingDown => ApiError::Unavailable,
    })?;
    Ok((StatusCode::ACCEPTED, Json(json!({ "id": id }))).into_response())
}

async fn get_job(
    State(pool): State<Pool>,
    Path(id): Path<String>,
) -> Result<Json<JobView>, ApiError> {
    pool.get(&id).map(Json).ok_or(ApiError::NotFound)
}

#[derive(Serialize)]
struct JobList {
    items: Vec<JobView>,
    total: usize,
}

async fn list_jobs(State(pool): State<Pool>) -> Json<JobList> {
    let items = pool.list();
    Json(JobList {
        total: items.len(),
        items,
    })
}

async fn cancel_job(
    State(pool): State<Pool>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    match pool.cancel(&id) {
        Ok(()) => Ok(StatusCode::NO_CONTENT),
        Err(CancelError::NotFound) => Err(ApiError::NotFound),
        Err(CancelError::Finished) => Err(ApiError::Conflict("job already finished".into())),
    }
}
