//! Helpers shared by the module 02 examples: where to listen and when to stop.

use std::net::SocketAddr;

/// Reads `ADDR` (`host:port`), defaulting to `127.0.0.1:8080`.
pub fn bind_addr() -> SocketAddr {
    let raw = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".to_string());
    raw.parse()
        .unwrap_or_else(|err| panic!("ADDR must look like host:port, got {raw:?}: {err}"))
}

/// Completes on Ctrl-C or (on Unix) SIGTERM; pass it to `with_graceful_shutdown`.
pub async fn shutdown_signal() {
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
