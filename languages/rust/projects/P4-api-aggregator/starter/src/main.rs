use std::time::Duration;

use p4_api_aggregator::{Config, build_app};
use tokio::net::TcpListener;

fn env_or<T: std::str::FromStr>(name: &str, default: T) -> T {
    std::env::var(name)
        .ok()
        .and_then(|v| v.parse().ok())
        .unwrap_or(default)
}

fn config_from_env() -> Config {
    let base = std::env::var("UPSTREAM_BASE").unwrap_or_else(|_| "http://127.0.0.1:9000".into());
    let mut config = Config::new(base);
    config.max_concurrency = env_or("MAX_CONCURRENCY", config.max_concurrency);
    config.max_retries = env_or("MAX_RETRIES", config.max_retries);
    config.call_timeout = Duration::from_millis(env_or("CALL_TIMEOUT_MS", 2_000));
    config.backoff_base = Duration::from_millis(env_or("BACKOFF_BASE_MS", 50));
    config.overall_timeout = Duration::from_millis(env_or("OVERALL_TIMEOUT_MS", 10_000));
    config
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let addr = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".into());
    let listener = TcpListener::bind(&addr).await?;
    eprintln!("aggregator listening on http://{}", listener.local_addr()?);
    axum::serve(listener, build_app(config_from_env()))
        .with_graceful_shutdown(shutdown_signal())
        .await
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let terminate = async {
        if let Ok(mut sig) =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
        {
            sig.recv().await;
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
}
