//! P7: a small, well-tested Tasks service with operational endpoints.
//!
//! * [`tasks`]: the Task model, the [`tasks::TaskStore`] trait and the handlers (given)
//! * [`probe`]: a tiny `GET /healthz` client for Docker's `HEALTHCHECK` (given)
//! * [`config`]: environment configuration with validation (your work)
//! * [`ops`]: `/readyz`, `/metrics` and the request counter (your work)
//!
//! Your other work is the packaging: `Dockerfile`, `.dockerignore` and `.github/workflows/ci.yml`.

pub mod config;
pub mod ops;
pub mod probe;
pub mod tasks;

use std::sync::Arc;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};

use axum::routing::get;
use axum::{Json, Router, middleware};
use serde_json::{Value, json};

use crate::tasks::{ApiError, TaskStore};

/// Everything the handlers share. Cloning is cheap: every field is an `Arc`.
#[derive(Clone)]
pub struct AppState {
    pub store: Arc<dyn TaskStore>,
    ready: Arc<AtomicBool>,
    requests: Arc<AtomicU64>,
}

impl AppState {
    /// A new state that is **not ready** yet and has counted no requests.
    pub fn new(store: Arc<dyn TaskStore>) -> Self {
        Self {
            store,
            ready: Arc::new(AtomicBool::new(false)),
            requests: Arc::new(AtomicU64::new(0)),
        }
    }

    pub fn set_ready(&self, ready: bool) {
        self.ready.store(ready, Ordering::Release);
    }

    pub fn is_ready(&self) -> bool {
        self.ready.load(Ordering::Acquire)
    }

    pub fn count_request(&self) {
        self.requests.fetch_add(1, Ordering::Relaxed);
    }

    pub fn requests_total(&self) -> u64 {
        self.requests.load(Ordering::Relaxed)
    }
}

/// The whole HTTP application. Tests build it from a state they keep a clone of.
pub fn build_app(state: AppState) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .merge(tasks::routes())
        .merge(ops::routes())
        .fallback(|| async { ApiError::NotFound })
        .layer(middleware::from_fn_with_state(
            state.clone(),
            ops::count_requests,
        ))
        .with_state(state)
}

/// Liveness: the process is up. Readiness is `/readyz`.
async fn healthz() -> Json<Value> {
    Json(json!({ "status": "ok" }))
}
