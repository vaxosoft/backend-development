use std::process::ExitCode;
use std::sync::Arc;

use p7_tested_service_docker_ci::config::Config;
use p7_tested_service_docker_ci::probe::probe;
use p7_tested_service_docker_ci::tasks::MemoryStore;
use p7_tested_service_docker_ci::{AppState, build_app};
use tokio::net::TcpListener;

fn main() -> ExitCode {
    // An invalid configuration stops the process before anything is bound.
    let cfg = match Config::from_env() {
        Ok(cfg) => cfg,
        Err(err) => {
            eprintln!("configuration error: {err}");
            return ExitCode::from(2);
        }
    };

    if std::env::args().nth(1).as_deref() == Some("healthcheck") {
        return if probe(&format!("127.0.0.1:{}", cfg.port)) {
            ExitCode::SUCCESS
        } else {
            ExitCode::FAILURE
        };
    }

    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::from(cfg.log_level))
        .init();

    let runtime = tokio::runtime::Runtime::new().expect("the Tokio runtime starts");
    match runtime.block_on(serve(cfg)) {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("server error: {err}");
            ExitCode::FAILURE
        }
    }
}

async fn serve(cfg: Config) -> std::io::Result<()> {
    let addr = std::env::var("ADDR").unwrap_or_else(|_| cfg.listen_addr());
    let listener = TcpListener::bind(&addr).await?;
    let state = AppState::new(Arc::new(MemoryStore::default()));
    state.set_ready(true);
    tracing::info!(%addr, env = ?cfg.env, "listening");

    let shutdown_state = state.clone();
    axum::serve(listener, build_app(state))
        .with_graceful_shutdown(async move {
            shutdown_signal().await;
            shutdown_state.set_ready(false);
            tracing::info!("shutting down");
        })
        .await
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let terminate = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut sig) => {
                sig.recv().await;
            }
            Err(_) => std::future::pending::<()>().await,
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {}
        _ = terminate => {}
    }
}
