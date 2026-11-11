//! Task Tracker service shaped for production: probes, version endpoint, SIGTERM handling and a
//! `--healthcheck` mode so a distroless image needs no curl.
//!
//! ADDR=127.0.0.1:20710 cargo run -p m14-packaging-and-running --example tracker_service

use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use axum::extract::State;
use axum::http::StatusCode;
use axum::routing::get;
use axum::{Json, Router};
use clap::Parser;
use m14_packaging_and_running::BUILD;
use serde_json::{Value, json};

#[derive(Debug, Parser)]
#[command(name = "tracker_service", version)]
struct Settings {
    #[arg(long, env = "ADDR", default_value = "127.0.0.1:8080")]
    addr: SocketAddr,
    #[arg(long, env = "WORKERS", default_value_t = 2, value_parser = clap::value_parser!(u16).range(1..=256))]
    workers: u16,
    #[arg(long, env = "SHUTDOWN_TIMEOUT_SECS", default_value_t = 5)]
    shutdown_timeout_secs: u64,
    /// Probe `GET /healthz` on --addr and exit 0 (healthy) or 1; used by the Docker HEALTHCHECK
    #[arg(long)]
    healthcheck: bool,
}

#[derive(Clone)]
struct AppState {
    ready: Arc<AtomicBool>,
}

async fn healthz() -> &'static str {
    "ok"
}

async fn readyz(State(app): State<AppState>) -> (StatusCode, &'static str) {
    if app.ready.load(Ordering::Acquire) {
        (StatusCode::OK, "ready")
    } else {
        (StatusCode::SERVICE_UNAVAILABLE, "starting")
    }
}

async fn version() -> Json<Value> {
    Json(json!({"name": BUILD.name, "version": BUILD.version, "profile": BUILD.profile}))
}

async fn tasks() -> Json<Value> {
    Json(json!({"items": [{"id": 1, "title": "Write docs", "status": "todo"}], "total": 1}))
}

fn router(state: AppState) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/readyz", get(readyz))
        .route("/version", get(version))
        .route("/tasks", get(tasks))
        .with_state(state)
}

/// Plain std TCP: a health probe must not depend on the async runtime being healthy.
fn probe(addr: SocketAddr) -> bool {
    let Ok(mut stream) = TcpStream::connect_timeout(&addr, Duration::from_secs(2)) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_secs(2)));
    let request = format!("GET /healthz HTTP/1.0\r\nHost: {addr}\r\n\r\n");
    let mut response = String::new();
    stream.write_all(request.as_bytes()).is_ok()
        && stream.read_to_string(&mut response).is_ok()
        && response.split_whitespace().nth(1) == Some("200")
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let sigterm = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut s) => {
                s.recv().await;
            }
            Err(_) => std::future::pending::<()>().await,
        }
    };
    #[cfg(not(unix))]
    let sigterm = std::future::pending::<()>();
    tokio::select! { _ = ctrl_c => {}, _ = sigterm => {} }
}

async fn serve(settings: Settings) -> anyhow::Result<()> {
    let ready = Arc::new(AtomicBool::new(false));
    let app = router(AppState {
        ready: ready.clone(),
    });
    let listener = tokio::net::TcpListener::bind(settings.addr).await?;
    println!(
        "{} {} ({}) listening on {}",
        BUILD.name,
        BUILD.version,
        BUILD.profile,
        listener.local_addr()?
    );
    ready.store(true, Ordering::Release);

    let (stop_tx, stop_rx) = tokio::sync::oneshot::channel::<()>();
    let server = axum::serve(listener, app).with_graceful_shutdown(async {
        let _ = stop_rx.await;
    });
    let mut server = Box::pin(server.into_future());

    tokio::select! {
        result = &mut server => return Ok(result?),
        _ = shutdown_signal() => {}
    }

    // Stop reporting ready first so a load balancer drains us, then let in-flight requests finish.
    ready.store(false, Ordering::Release);
    println!(
        "shutting down (up to {}s for in-flight requests)",
        settings.shutdown_timeout_secs
    );
    let _ = stop_tx.send(());
    match tokio::time::timeout(Duration::from_secs(settings.shutdown_timeout_secs), server).await {
        Ok(result) => result?,
        Err(_) => println!("shutdown timeout reached, exiting anyway"),
    }
    println!("bye");
    Ok(())
}

fn main() -> anyhow::Result<()> {
    let settings = Settings::parse();
    if settings.healthcheck {
        std::process::exit(if probe(settings.addr) { 0 } else { 1 });
    }

    // Building the runtime by hand makes the worker count a setting instead of "number of CPUs".
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .worker_threads(usize::from(settings.workers))
        .thread_name("tracker-worker")
        .enable_all()
        .build()?;
    runtime.block_on(serve(settings))
}
