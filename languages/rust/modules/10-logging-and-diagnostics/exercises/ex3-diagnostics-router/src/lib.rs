//! Exercise 3: health, metrics and panic recovery for the Task Tracker API.
use std::any::Any;
use std::sync::Arc;
use std::sync::atomic::AtomicU64;

use axum::{
    Json, Router,
    body::Body,
    extract::{Request, State},
    http::StatusCode,
    middleware::Next,
    response::Response,
    routing::get,
};

/// Request counters shared by all clones (they are handles to the same atomics).
#[derive(Clone, Default)]
pub struct Metrics {
    #[allow(dead_code)] // the fields are used by your implementation
    total: Arc<AtomicU64>,
    #[allow(dead_code)]
    client_errors: Arc<AtomicU64>,
    #[allow(dead_code)]
    server_errors: Arc<AtomicU64>,
}

impl Metrics {
    /// Count one finished response: always in the total, and also in the 4xx or 5xx counter.
    pub fn record(&self, _status: StatusCode) {
        todo!("increment the right atomics")
    }

    /// Text exposition, exactly these three lines (each ending in `\n`):
    ///
    /// ```text
    /// http_requests_total 5
    /// http_client_errors_total 1
    /// http_server_errors_total 1
    /// ```
    pub fn render(&self) -> String {
        todo!("load the counters and format them")
    }
}

/// Middleware: let the request through, then call [`Metrics::record`] with the response status.
pub async fn count_requests(
    State(_metrics): State<Metrics>,
    _request: Request,
    _next: Next,
) -> Response {
    todo!("run the rest of the stack, record the status")
}

/// Turn a handler panic into a response:
/// - log an ERROR event with the message `"handler panicked"` and the field `panic` holding the
///   panic message (a `&str` or `String` payload; use `"unknown panic"` for anything else)
/// - answer `500` with `content-type: application/json` and the body
///   `{"error":{"code":"internal","message":"internal server error"}}`. The panic message
///   must NOT be sent to the client.
pub fn panic_response(_payload: Box<dyn Any + Send + 'static>) -> Response<Body> {
    todo!("log it, hide it")
}

/// Routes (given): `GET /healthz`, `GET /metrics`, `GET /boom`.
///
/// Add the layers so that:
/// - a panic in any handler becomes the response of [`panic_response`] and the server keeps
///   serving
/// - EVERY response is counted, including the 500 produced from a panic. Layers wrap the ones
///   added before them: decide which of the two has to be outermost.
pub fn build_app(metrics: Metrics) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/metrics", get(metrics_text))
        .route("/boom", get(boom))
        .with_state(metrics)
}

async fn healthz() -> Json<serde_json::Value> {
    Json(serde_json::json!({"status": "ok"}))
}

async fn metrics_text(State(metrics): State<Metrics>) -> String {
    metrics.render()
}

async fn boom() -> &'static str {
    panic!("kaboom");
}
