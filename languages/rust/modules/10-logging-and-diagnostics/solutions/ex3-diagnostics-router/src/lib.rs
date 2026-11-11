//! Exercise 3: health, metrics and panic recovery for the Task Tracker API.
use std::any::Any;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use axum::{
    Json, Router,
    body::Body,
    extract::{Request, State},
    http::{StatusCode, header},
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::get,
};
use tower_http::catch_panic::CatchPanicLayer;
use tracing::error;

/// Request counters shared by all clones (they are handles to the same atomics).
#[derive(Clone, Default)]
pub struct Metrics {
    total: Arc<AtomicU64>,
    client_errors: Arc<AtomicU64>,
    server_errors: Arc<AtomicU64>,
}

impl Metrics {
    /// Count one finished response: always in the total, and also in the 4xx or 5xx counter.
    pub fn record(&self, status: StatusCode) {
        self.total.fetch_add(1, Ordering::Relaxed);
        if status.is_client_error() {
            self.client_errors.fetch_add(1, Ordering::Relaxed);
        } else if status.is_server_error() {
            self.server_errors.fetch_add(1, Ordering::Relaxed);
        }
    }

    /// Text exposition, exactly these three lines (each ending in `\n`):
    ///
    /// ```text
    /// http_requests_total 5
    /// http_client_errors_total 1
    /// http_server_errors_total 1
    /// ```
    pub fn render(&self) -> String {
        format!(
            "http_requests_total {}\nhttp_client_errors_total {}\nhttp_server_errors_total {}\n",
            self.total.load(Ordering::Relaxed),
            self.client_errors.load(Ordering::Relaxed),
            self.server_errors.load(Ordering::Relaxed)
        )
    }
}

/// Middleware: let the request through, then call [`Metrics::record`] with the response status.
pub async fn count_requests(
    State(metrics): State<Metrics>,
    request: Request,
    next: Next,
) -> Response {
    let response = next.run(request).await;
    metrics.record(response.status());
    response
}

/// Turn a handler panic into a response:
/// - log an ERROR event with the message `"handler panicked"` and the field `panic` holding the
///   panic message (a `&str` or `String` payload; use `"unknown panic"` for anything else)
/// - answer `500` with `content-type: application/json` and the body
///   `{"error":{"code":"internal","message":"internal server error"}}`. The panic message
///   must NOT be sent to the client.
pub fn panic_response(payload: Box<dyn Any + Send + 'static>) -> Response<Body> {
    let message = payload
        .downcast_ref::<&str>()
        .map(|s| (*s).to_owned())
        .or_else(|| payload.downcast_ref::<String>().cloned())
        .unwrap_or_else(|| "unknown panic".to_owned());
    error!(panic = %message, "handler panicked");

    let body =
        serde_json::json!({"error": {"code": "internal", "message": "internal server error"}});
    (
        StatusCode::INTERNAL_SERVER_ERROR,
        [(header::CONTENT_TYPE, "application/json")],
        body.to_string(),
    )
        .into_response()
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
        .with_state(metrics.clone())
        .layer(CatchPanicLayer::custom(panic_response))
        .layer(middleware::from_fn_with_state(metrics, count_requests))
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
