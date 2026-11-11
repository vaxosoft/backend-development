//! Exercise 3: graceful shutdown with a grace period.
//!
//! `serve` runs `app` on `listener` until `shutdown` completes. Then:
//! * the listener stops accepting new connections,
//! * in-flight requests are allowed to finish,
//! * but if they are not done within `grace`, `serve` gives up and returns
//!   `Outcome::TimedOut` anyway (a stuck handler must not block the process).

use std::future::Future;
use std::io;
use std::sync::Arc;
use std::time::Duration;

use axum::Router;
use axum::extract::State;
use axum::routing::get;
use tokio::net::TcpListener;
use tokio::sync::Notify;

#[derive(Debug, PartialEq, Eq)]
pub enum Outcome {
    /// Every in-flight request finished within the grace period.
    Clean,
    /// The grace period ran out first.
    TimedOut,
}

/// Test hooks for `GET /wait`: the handler signals `entered`, then blocks until
/// `release` is notified. Already implemented below.
#[derive(Clone, Default)]
pub struct Hooks {
    pub entered: Arc<Notify>,
    pub release: Arc<Notify>,
}

async fn wait(State(hooks): State<Hooks>) -> &'static str {
    hooks.entered.notify_one();
    hooks.release.notified().await;
    "released"
}

pub fn router(hooks: Hooks) -> Router {
    Router::new()
        .route("/ping", get(|| async { "pong" }))
        .route("/wait", get(wait))
        .with_state(hooks)
}

pub async fn serve(
    _listener: TcpListener,
    _app: Router,
    _shutdown: impl Future<Output = ()> + Send + 'static,
    _grace: Duration,
) -> io::Result<Outcome> {
    todo!("axum::serve(..).with_graceful_shutdown(..), plus a timer that starts when shutdown does")
}
