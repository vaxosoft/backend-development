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
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};

use axum::extract::{Path, Request, State};
use axum::http::{HeaderMap, HeaderName, HeaderValue, StatusCode};
use axum::middleware::{self, Next};
use axum::response::Response;
use axum::routing::{get, post};
use axum::{Json, Router};

#[derive(Clone, Default)]
pub struct AppState {
    pub counters: Arc<Mutex<HashMap<String, u64>>>,
    pub next_request: Arc<AtomicU64>,
}

static REQUEST_ID: HeaderName = HeaderName::from_static("x-request-id");

async fn request_id(State(state): State<AppState>, mut req: Request, next: Next) -> Response {
    let id = match req.headers().get(&REQUEST_ID) {
        Some(value) if !value.is_empty() => value.clone(),
        _ => {
            let n = state.next_request.fetch_add(1, Ordering::Relaxed) + 1;
            HeaderValue::from_str(&format!("req-{n}")).expect("ascii digits are a valid header")
        }
    };
    req.headers_mut().insert(REQUEST_ID.clone(), id.clone());
    let mut response = next.run(req).await;
    response.headers_mut().insert(REQUEST_ID.clone(), id);
    response
}

async fn whoami(headers: HeaderMap) -> String {
    headers
        .get(&REQUEST_ID)
        .and_then(|v| v.to_str().ok())
        .unwrap_or_default()
        .to_string()
}

async fn increment(
    State(state): State<AppState>,
    Path(name): Path<String>,
) -> Json<serde_json::Value> {
    let mut counters = state.counters.lock().expect("counters mutex poisoned");
    let count = counters.entry(name.clone()).or_insert(0);
    *count += 1;
    Json(serde_json::json!({ "name": name, "count": *count }))
}

async fn read_counter(
    State(state): State<AppState>,
    Path(name): Path<String>,
) -> Result<Json<serde_json::Value>, StatusCode> {
    let counters = state.counters.lock().expect("counters mutex poisoned");
    let count = counters.get(&name).ok_or(StatusCode::NOT_FOUND)?;
    Ok(Json(serde_json::json!({ "name": name, "count": count })))
}

pub fn router() -> Router {
    let state = AppState::default();
    Router::new()
        .route("/whoami", get(whoami))
        .route("/counters/{name}", post(increment).get(read_counter))
        .layer(middleware::from_fn_with_state(state.clone(), request_id))
        .with_state(state)
}
