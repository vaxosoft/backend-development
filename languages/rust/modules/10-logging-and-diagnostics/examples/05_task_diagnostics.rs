//! Task Tracker diagnostics: tower-http request ids + `TraceLayer`, panic recovery as a 500
//! error envelope, a health endpoint and atomic-counter metrics.
//! Listens on ADDR (default 127.0.0.1:8080).
use std::net::SocketAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use axum::{
    Json, Router,
    body::Body,
    extract::{Path, Request, State},
    http::StatusCode,
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::get,
};
use serde_json::json;
use tower_http::{
    catch_panic::CatchPanicLayer,
    request_id::{MakeRequestId, PropagateRequestIdLayer, RequestId, SetRequestIdLayer},
    trace::{DefaultOnResponse, TraceLayer},
};
use tracing::{Level, info, info_span};
use tracing_subscriber::EnvFilter;

#[derive(Clone, Default)]
struct Metrics {
    requests: Arc<AtomicU64>,
    server_errors: Arc<AtomicU64>,
}

/// Generates `req-0001`, `req-0002`, ... when the caller did not send `X-Request-ID`.
#[derive(Clone, Default)]
struct SequentialIds(Arc<AtomicU64>);

impl MakeRequestId for SequentialIds {
    fn make_request_id<B>(&mut self, _request: &axum::http::Request<B>) -> Option<RequestId> {
        let n = self.0.fetch_add(1, Ordering::Relaxed) + 1;
        format!("req-{n:04}").parse().ok().map(RequestId::new)
    }
}

async fn count_requests(State(metrics): State<Metrics>, request: Request, next: Next) -> Response {
    let response = next.run(request).await;
    metrics.requests.fetch_add(1, Ordering::Relaxed);
    if response.status().is_server_error() {
        metrics.server_errors.fetch_add(1, Ordering::Relaxed);
    }
    response
}

async fn healthz() -> Json<serde_json::Value> {
    Json(json!({"status": "ok"}))
}

async fn metrics_text(State(metrics): State<Metrics>) -> String {
    format!(
        "http_requests_total {}\nhttp_server_errors_total {}\n",
        metrics.requests.load(Ordering::Relaxed),
        metrics.server_errors.load(Ordering::Relaxed)
    )
}

async fn get_task(Path(id): Path<u64>) -> Response {
    if id == 1 {
        info!(task_id = id, "task found");
        return Json(json!({"id": 1, "title": "Write docs", "status": "todo"})).into_response();
    }
    envelope(StatusCode::NOT_FOUND, "not_found", "task not found")
}

async fn boom() -> &'static str {
    panic!("something impossible happened");
}

fn envelope(status: StatusCode, code: &str, message: &str) -> Response {
    (
        status,
        Json(json!({"error": {"code": code, "message": message}})),
    )
        .into_response()
}

/// Never leak the panic message to the client; the panic hook already logged it.
fn panic_response(_panic: Box<dyn std::any::Any + Send + 'static>) -> Response<Body> {
    envelope(
        StatusCode::INTERNAL_SERVER_ERROR,
        "internal",
        "internal server error",
    )
}

fn build_app(metrics: Metrics) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/metrics", get(metrics_text))
        .route("/tasks/{id}", get(get_task))
        .route("/panic", get(boom))
        .with_state(metrics.clone())
        // Layers wrap what was added before them, so read this list from the bottom up:
        // a request meets SetRequestId first and the handler last.
        .layer(CatchPanicLayer::custom(panic_response))
        .layer(middleware::from_fn_with_state(metrics, count_requests))
        .layer(PropagateRequestIdLayer::x_request_id())
        .layer(
            TraceLayer::new_for_http()
                .make_span_with(|request: &Request| {
                    let request_id = request.headers().get("x-request-id").and_then(|v| v.to_str().ok()).unwrap_or("-");
                    info_span!("request", request_id, method = %request.method(), path = %request.uri().path())
                })
                .on_response(DefaultOnResponse::new().level(Level::INFO)),
        )
        .layer(SetRequestIdLayer::x_request_id(SequentialIds::default()))
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .without_time() // timestamps are off only so this lesson's output is stable
        .with_target(false)
        .with_ansi(false)
        .init();

    // Panics become structured log events instead of raw text on stderr.
    std::panic::set_hook(Box::new(|info| {
        let location = info
            .location()
            .map(|l| format!("{}:{}", l.file(), l.line()))
            .unwrap_or_default();
        tracing::error!(panic = info.payload_as_str().unwrap_or("non-string payload"), %location, "handler panicked");
    }));

    let addr: SocketAddr = std::env::var("ADDR")
        .unwrap_or_else(|_| "127.0.0.1:8080".into())
        .parse()?;
    let listener = tokio::net::TcpListener::bind(addr).await?;
    info!(%addr, "listening");
    axum::serve(listener, build_app(Metrics::default()))
        .with_graceful_shutdown(shutdown_signal())
        .await?;
    info!("shut down cleanly");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("install Ctrl-C handler")
    };
    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("install SIGTERM handler")
            .recv()
            .await;
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! {
        _ = ctrl_c => {},
        _ = terminate => {},
    }
}
