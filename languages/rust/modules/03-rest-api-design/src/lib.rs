//! Module 03: REST API design. Shared building blocks for the examples.
//!
//! * [`error`]: the `ApiError` envelope and extractors that report failures with it.
//! * [`tasks`]: Task Tracker types, validation (create and PATCH) and an in-memory store.
//! * [`list`]: filtering, sorting and pagination for `GET /tasks`.

pub mod error;
pub mod list;
pub mod tasks;

use std::net::SocketAddr;

use axum::Router;
use tokio::net::TcpListener;

pub use error::{ApiError, ApiJson, ApiPath, ApiQuery, FieldErrors, not_found_fallback};

/// Reads `ADDR` (`host:port`), defaulting to `127.0.0.1:8080`.
pub fn bind_addr() -> SocketAddr {
    let raw = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".to_string());
    raw.parse()
        .unwrap_or_else(|err| panic!("ADDR must look like host:port, got {raw:?}: {err}"))
}

/// Serves `app` on `ADDR` until Ctrl-C or SIGTERM, then drains in-flight requests.
pub async fn run(app: Router) -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await
}

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl-C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("failed to install SIGTERM handler")
            .recv()
            .await;
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
    eprintln!("shutdown signal received, draining connections");
}
