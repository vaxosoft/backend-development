//! Task Tracker server: shared state, JSON CRUD subset, timeouts, graceful shutdown.
//!
//! ```text
//! ADDR=127.0.0.1:20404 cargo run -p m02-http-server --example tasks_server
//! ```

use std::collections::BTreeMap;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use axum::extract::{Path, State};
use axum::http::{StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use m02_http_server::{bind_addr, shutdown_signal};
use serde::{Deserialize, Serialize};
use tokio::net::TcpListener;
use tower_http::timeout::TimeoutLayer;

#[derive(Clone, Serialize)]
struct Task {
    id: u32,
    title: String,
    status: String,
    created_at: String,
}

#[derive(Deserialize)]
struct NewTask {
    title: String,
}

#[derive(Default)]
struct Store {
    next_id: u32,
    tasks: BTreeMap<u32, Task>,
}

// The lock is a plain `std::sync::Mutex`: handlers lock, copy what they need and
// release before any `.await`, so the guard never lives across a suspension point.
type Db = Arc<Mutex<Store>>;

async fn healthz() -> Json<serde_json::Value> {
    Json(serde_json::json!({ "status": "ok" }))
}

async fn list(State(db): State<Db>) -> Json<Vec<Task>> {
    let store = db.lock().expect("store mutex poisoned");
    Json(store.tasks.values().cloned().collect())
}

async fn create(State(db): State<Db>, Json(input): Json<NewTask>) -> Response {
    let mut store = db.lock().expect("store mutex poisoned");
    store.next_id += 1;
    let task = Task {
        id: store.next_id,
        title: input.title.trim().to_string(),
        status: "todo".to_string(),
        // A fixed stand-in keeps the output reproducible; real clocks arrive in module 13.
        created_at: "2030-01-01T09:00:00Z".to_string(),
    };
    store.tasks.insert(task.id, task.clone());
    let location = format!("/tasks/{}", task.id);
    (
        StatusCode::CREATED,
        [(header::LOCATION, location)],
        Json(task),
    )
        .into_response()
}

async fn get_one(State(db): State<Db>, Path(id): Path<u32>) -> Result<Json<Task>, StatusCode> {
    let store = db.lock().expect("store mutex poisoned");
    store
        .tasks
        .get(&id)
        .cloned()
        .map(Json)
        .ok_or(StatusCode::NOT_FOUND)
}

async fn delete_one(State(db): State<Db>, Path(id): Path<u32>) -> StatusCode {
    let mut store = db.lock().expect("store mutex poisoned");
    if store.tasks.remove(&id).is_some() {
        StatusCode::NO_CONTENT
    } else {
        StatusCode::NOT_FOUND
    }
}

fn app(db: Db) -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route("/tasks", get(list).post(create))
        .route("/tasks/{id}", get(get_one).delete(delete_one))
        .layer(TimeoutLayer::with_status_code(
            StatusCode::REQUEST_TIMEOUT,
            Duration::from_secs(5),
        ))
        .with_state(db)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app(Db::default()))
        .with_graceful_shutdown(shutdown_signal())
        .await
}
