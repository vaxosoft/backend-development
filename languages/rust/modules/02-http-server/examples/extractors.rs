//! Extractors: `State`, `Path`, `Query`, `HeaderMap`, `Json`, `Form`, `Bytes`
//! and `Multipart`. Body-consuming extractors must come last.
//!
//! ```text
//! ADDR=127.0.0.1:20401 cargo run -p m02-http-server --example extractors
//! ```

use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use axum::body::Bytes;
use axum::extract::{Multipart, Path, Query, State};
use axum::http::{HeaderMap, StatusCode, header};
use axum::routing::{get, post, put};
use axum::{Form, Json, Router};
use m02_http_server::{bind_addr, shutdown_signal};
use serde::Deserialize;
use tokio::net::TcpListener;

// `State` must be `Clone`; an `Arc` makes the clone cheap and shares the data.
#[derive(Clone, Default)]
struct AppState {
    hits: Arc<AtomicU64>,
}

#[derive(Deserialize)]
struct Search {
    q: String,
    // Repeated keys (`?tag=a&tag=b`) do not deserialize into a Vec with `Query`;
    // accept a comma separated value instead.
    tags: Option<String>,
    limit: Option<u32>,
}

#[derive(Deserialize)]
struct TaskInput {
    title: String,
}

#[derive(Deserialize)]
struct LoginForm {
    email: String,
}

async fn counter(State(state): State<AppState>) -> String {
    let n = state.hits.fetch_add(1, Ordering::Relaxed) + 1;
    format!("hit {n}\n")
}

async fn get_item(Path(id): Path<u32>) -> String {
    format!("item {id}\n")
}

async fn search(Query(s): Query<Search>) -> String {
    let tags: Vec<&str> = s.tags.as_deref().map_or(vec![], |t| t.split(',').collect());
    format!("q={} tags={tags:?} limit={}\n", s.q, s.limit.unwrap_or(20))
}

async fn headers(headers: HeaderMap) -> String {
    let agent = headers
        .get(header::USER_AGENT)
        .and_then(|v| v.to_str().ok())
        .unwrap_or("unknown");
    format!("user-agent: {agent}\n")
}

// Order matters: `State` and `Path` only read the request head, `Json` consumes
// the body, so it has to be the last parameter.
async fn replace_task(
    State(state): State<AppState>,
    Path(id): Path<u32>,
    Json(input): Json<TaskInput>,
) -> Json<serde_json::Value> {
    state.hits.fetch_add(1, Ordering::Relaxed);
    Json(serde_json::json!({ "id": id, "title": input.title.trim() }))
}

async fn login(Form(form): Form<LoginForm>) -> String {
    format!("form email: {}\n", form.email)
}

async fn raw(body: Bytes) -> String {
    format!("received {} bytes\n", body.len())
}

async fn upload(mut multipart: Multipart) -> Result<String, (StatusCode, String)> {
    let mut report = String::new();
    while let Some(field) = multipart
        .next_field()
        .await
        .map_err(|e| (e.status(), e.body_text()))?
    {
        let name = field.name().unwrap_or("?").to_string();
        let file_name = field.file_name().map(str::to_string);
        let data = field
            .bytes()
            .await
            .map_err(|e| (e.status(), e.body_text()))?;
        report.push_str(&format!("{name} file={file_name:?} {} bytes\n", data.len()));
    }
    Ok(report)
}

fn app() -> Router {
    Router::new()
        .route("/counter", get(counter))
        .route("/items/{id}", get(get_item))
        .route("/search", get(search))
        .route("/headers", get(headers))
        .route("/tasks/{id}", put(replace_task))
        .route("/login", post(login))
        .route("/raw", post(raw))
        .route("/upload", post(upload))
        .with_state(AppState::default())
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app())
        .with_graceful_shutdown(shutdown_signal())
        .await
}
