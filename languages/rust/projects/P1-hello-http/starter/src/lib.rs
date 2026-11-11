//! P1: a small HTTP server with routing, middleware and graceful shutdown.

pub mod error;
pub mod handlers;
pub mod middleware;

use std::future::Future;
use std::io;
use std::sync::Arc;
use std::time::Duration;

use axum::Router;
use axum::routing::get;
use tokio::net::TcpListener;

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
pub fn build_app(_config: Config) -> Router {
    // TODO: add the layers. Order matters: the request ID must be outermost so
    // the log line can read it, the access log must see the status produced by
    // the panic handler, and the timeout sits closest to the handlers.
    Router::new()
        .route("/healthz", get(handlers::healthz))
        .route("/hello", get(handlers::hello_world))
        .route("/hello/{name}", get(handlers::hello))
        .route("/slow", get(handlers::slow))
        .route("/panic", get(handlers::boom))
}

/// Serves `app` on `listener` until `shutdown` resolves.
///
/// TODO: shut down gracefully: stop accepting new connections, let in-flight
/// requests finish, then return.
pub async fn run(
    listener: TcpListener,
    app: Router,
    _shutdown: impl Future<Output = ()> + Send + 'static,
) -> io::Result<()> {
    axum::serve(listener, app).await
}
