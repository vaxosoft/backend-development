//! Exercise 3: graceful shutdown with a grace period.
//!
//! `serve` runs `app` on `listener` until `shutdown` completes. Then:
//! * the listener stops accepting new connections,
//! * in-flight requests are allowed to finish,
//! * but if they are not done within `grace`, `serve` gives up and returns
//!   `Outcome::TimedOut` anyway (a stuck handler must not block the process).

use std::future::{Future, IntoFuture};
use std::io;
use std::sync::Arc;
use std::time::Duration;

use axum::Router;
use axum::extract::State;
use axum::routing::get;
use tokio::net::TcpListener;
use tokio::sync::{Notify, oneshot};

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
    listener: TcpListener,
    app: Router,
    shutdown: impl Future<Output = ()> + Send + 'static,
    grace: Duration,
) -> io::Result<Outcome> {
    // Tells the grace timer when draining has begun.
    let (began_tx, began_rx) = oneshot::channel::<()>();
    let server = axum::serve(listener, app)
        .with_graceful_shutdown(async move {
            shutdown.await;
            let _ = began_tx.send(());
        })
        .into_future();
    tokio::pin!(server);

    let grace_expired = async {
        if began_rx.await.is_ok() {
            tokio::time::sleep(grace).await;
        } else {
            std::future::pending::<()>().await;
        }
    };

    tokio::select! {
        result = &mut server => result.map(|()| Outcome::Clean),
        () = grace_expired => Ok(Outcome::TimedOut),
    }
}
