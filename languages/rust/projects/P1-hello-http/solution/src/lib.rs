//! P1: a small HTTP server with routing, middleware and graceful shutdown.

pub mod error;
pub mod handlers;
pub mod middleware;

use std::future::Future;
use std::io;
use std::sync::Arc;
use std::time::Duration;

use axum::Router;
use axum::http::StatusCode;
use axum::middleware::{from_fn, from_fn_with_state};
use axum::routing::get;
use tokio::net::TcpListener;
use tower_http::catch_panic::CatchPanicLayer;
use tower_http::timeout::TimeoutLayer;

/// Receives one formatted access-log line per request.
pub type LogSink = Arc<dyn Fn(&str) + Send + Sync>;

#[derive(Clone)]
pub struct Config {
    pub log_sink: LogSink,
    /// Requests that take longer than this are answered with 408.
    pub request_timeout: Duration,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            log_sink: Arc::new(|line| println!("{line}")),
            request_timeout: Duration::from_secs(10),
        }
    }
}

/// Builds the router with all routes and middleware.
pub fn build_app(config: Config) -> Router {
    // The last layer added is the outermost one.
    Router::new()
        .route("/healthz", get(handlers::healthz))
        .route("/hello", get(handlers::hello_world))
        .route("/hello/{name}", get(handlers::hello))
        .route("/slow", get(handlers::slow))
        .route("/panic", get(handlers::boom))
        .layer(TimeoutLayer::with_status_code(
            StatusCode::REQUEST_TIMEOUT,
            config.request_timeout,
        ))
        .layer(CatchPanicLayer::custom(middleware::panic_response))
        .layer(from_fn_with_state(config.log_sink, middleware::access_log))
        .layer(from_fn(middleware::request_id))
}

/// Serves `app` on `listener` until `shutdown` resolves.
///
/// Graceful: the listener is closed, in-flight requests finish, then it returns.
pub async fn run(
    listener: TcpListener,
    app: Router,
    shutdown: impl Future<Output = ()> + Send + 'static,
) -> io::Result<()> {
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown)
        .await
}
