//! Exercise 2: `State`, `from_fn` middleware and mutating request/response headers.
//!
//! * Middleware `request_id`: use the incoming `x-request-id` header when it is
//!   present and non-empty, otherwise generate `req-1`, `req-2`, ... from the
//!   shared counter. The id must be visible to handlers (put it on the request
//!   headers) and be returned in the `x-request-id` response header.
//! * `GET /whoami` answers with the request id as plain text.
//! * `POST /counters/{name}` increments a named counter and answers
//!   `{"name":"...","count":N}`; `GET /counters/{name}` reads it (`404` if the
//!   counter was never incremented).

use std::collections::HashMap;
use std::sync::atomic::AtomicU64;
use std::sync::{Arc, Mutex};

use axum::extract::{Path, Request, State};
use axum::http::{HeaderMap, StatusCode};
use axum::middleware::{self, Next};
use axum::response::Response;
use axum::routing::{get, post};
use axum::{Json, Router};

#[derive(Clone, Default)]
pub struct AppState {
    pub counters: Arc<Mutex<HashMap<String, u64>>>,
    pub next_request: Arc<AtomicU64>,
}

async fn request_id(State(_state): State<AppState>, _req: Request, _next: Next) -> Response {
    todo!("pick or generate the id, store it on the request, run the rest, copy it to the response")
}

async fn whoami(_headers: HeaderMap) -> String {
    todo!("read the request id header")
}

async fn increment(
    State(_state): State<AppState>,
    Path(_name): Path<String>,
) -> Json<serde_json::Value> {
    todo!("lock, increment, release")
}

async fn read_counter(
    State(_state): State<AppState>,
    Path(_name): Path<String>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    todo!("return the counter or 404")
}

pub fn router() -> Router {
    let state = AppState::default();
    Router::new()
        .route("/whoami", get(whoami))
        .route("/counters/{name}", post(increment).get(read_counter))
        .layer(middleware::from_fn_with_state(state.clone(), request_id))
        .with_state(state)
}
