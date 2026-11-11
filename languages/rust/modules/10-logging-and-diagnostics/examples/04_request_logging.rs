//! Request logging middleware written by hand: a request id, a span per request, and one
//! access-log line with status and latency. Listens on ADDR (default 127.0.0.1:8080).
use std::net::SocketAddr;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Instant;

use axum::{
    Router,
    extract::{Request, State},
    http::{HeaderValue, StatusCode},
    middleware::{self, Next},
    response::Response,
    routing::get,
};
use tracing::{Instrument, error, info, info_span, warn};
use tracing_subscriber::EnvFilter;

#[derive(Clone, Default)]
struct RequestIds(Arc<AtomicU64>);

impl RequestIds {
    /// A counter keeps this lesson deterministic; production code uses a UUID (module 13).
    fn next(&self) -> String {
        format!("req-{:04}", self.0.fetch_add(1, Ordering::Relaxed) + 1)
    }
}

async fn request_logging(State(ids): State<RequestIds>, request: Request, next: Next) -> Response {
    // Reuse the caller's id when it is sane, so one id follows a request across services.
    let request_id = request
        .headers()
        .get("x-request-id")
        .and_then(|v| v.to_str().ok())
        .filter(|v| !v.is_empty() && v.len() <= 64 && v.chars().all(|c| c.is_ascii_graphic()))
        .map_or_else(|| ids.next(), str::to_owned);

    let span = info_span!("request", %request_id, method = %request.method(), path = %request.uri().path());
    let started = Instant::now();
    let mut response = next.run(request).instrument(span.clone()).await;

    let status = response.status();
    let latency_ms = started.elapsed().as_millis();
    span.in_scope(|| match status.as_u16() {
        500.. => error!(status = status.as_u16(), latency_ms, "request failed"),
        400..=499 => warn!(status = status.as_u16(), latency_ms, "request rejected"),
        _ => info!(status = status.as_u16(), latency_ms, "request completed"),
    });

    if let Ok(value) = HeaderValue::from_str(&request_id) {
        response.headers_mut().insert("x-request-id", value);
    }
    response
}

async fn hello() -> &'static str {
    // Inherits request_id, method and path from the span the middleware entered.
    info!("saying hello");
    "Hello, world!"
}

async fn fail() -> StatusCode {
    StatusCode::INTERNAL_SERVER_ERROR
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

    let app = Router::new()
        .route("/hello", get(hello))
        .route("/fail", get(fail))
        .layer(middleware::from_fn_with_state(
            RequestIds::default(),
            request_logging,
        ));

    let addr: SocketAddr = std::env::var("ADDR")
        .unwrap_or_else(|_| "127.0.0.1:8080".into())
        .parse()?;
    let listener = tokio::net::TcpListener::bind(addr).await?;
    info!(%addr, "listening");
    axum::serve(listener, app)
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
