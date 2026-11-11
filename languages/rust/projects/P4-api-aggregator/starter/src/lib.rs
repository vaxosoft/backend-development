//! P4: an API aggregator. `GET /aggregate?ids=1,2,3` fans out to an upstream
//! service concurrently and merges the answers.

pub mod aggregate;
pub mod ids;
pub mod upstream;

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;

use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use serde::Serialize;
use serde_json::{Value, json};

#[derive(Debug, Clone)]
pub struct Config {
    /// Base URL of the upstream, without a trailing slash: `{base}/items/{id}` is called.
    pub upstream_base: String,
    pub max_concurrency: usize,
    pub call_timeout: Duration,
    /// Extra attempts after the first one.
    pub max_retries: u32,
    /// Delay before the first retry; it doubles for every further retry.
    pub backoff_base: Duration,
    /// Deadline for the whole `/aggregate` request.
    pub overall_timeout: Duration,
}

impl Config {
    pub fn new(upstream_base: impl Into<String>) -> Self {
        Self {
            upstream_base: upstream_base.into(),
            max_concurrency: 4,
            call_timeout: Duration::from_secs(2),
            max_retries: 2,
            backoff_base: Duration::from_millis(50),
            overall_timeout: Duration::from_secs(10),
        }
    }
}

/// Cloned per request: `reqwest::Client` is a cheap handle and the config sits behind an `Arc`.
#[derive(Clone)]
pub struct AppState {
    pub client: reqwest::Client,
    pub config: Arc<Config>,
}

#[derive(Debug, Serialize)]
pub struct Success {
    pub id: u64,
    pub data: Value,
}

#[derive(Debug, Serialize)]
pub struct Failure {
    pub id: u64,
    pub error: String,
}

#[derive(Debug, Serialize, Default)]
pub struct AggregateResponse {
    pub results: Vec<Success>,
    pub errors: Vec<Failure>,
}

#[derive(Debug, thiserror::Error)]
#[error("{0}")]
pub struct BadRequest(pub String);

impl IntoResponse for BadRequest {
    fn into_response(self) -> Response {
        let body = json!({"error": {"code": "bad_request", "message": self.0}});
        (StatusCode::BAD_REQUEST, Json(body)).into_response()
    }
}

pub fn build_app(config: Config) -> Router {
    let state = AppState {
        client: reqwest::Client::new(),
        config: Arc::new(config),
    };
    Router::new()
        .route("/aggregate", get(aggregate_handler))
        .with_state(state)
}

async fn aggregate_handler(
    State(state): State<AppState>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<Json<AggregateResponse>, BadRequest> {
    let ids = ids::parse_ids(params.get("ids").map(String::as_str))?;
    Ok(Json(aggregate::run(&state, ids).await))
}
