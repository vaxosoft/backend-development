//! Routing, path and query parameters, a fallback, and graceful shutdown.
//!
//! ```text
//! ADDR=127.0.0.1:20400 cargo run -p m02-http-server --example hello
//! ```

use axum::Router;
use axum::extract::{Path, Query};
use axum::http::StatusCode;
use axum::routing::get;
use m02_http_server::{bind_addr, shutdown_signal};
use serde::Deserialize;
use tokio::net::TcpListener;

#[derive(Deserialize)]
struct Greeting {
    // `default` makes the parameter optional; a missing key is not an error.
    #[serde(default = "default_greeting")]
    greeting: String,
}

fn default_greeting() -> String {
    "Hello".to_string()
}

async fn index() -> &'static str {
    "Task Tracker\n"
}

async fn hello_world() -> &'static str {
    "Hello, world!\n"
}

async fn hello(Path(name): Path<String>, Query(q): Query<Greeting>) -> String {
    format!("{}, {name}!\n", q.greeting)
}

async fn user_task(Path((user, task)): Path<(u32, u32)>) -> String {
    format!("user {user}, task {task}\n")
}

async fn static_like(Path(rest): Path<String>) -> String {
    format!("wildcard captured '{rest}'\n")
}

async fn not_found() -> (StatusCode, &'static str) {
    (StatusCode::NOT_FOUND, "nothing here\n")
}

fn app() -> Router {
    Router::new()
        .route("/", get(index))
        .route("/hello", get(hello_world))
        .route("/hello/{name}", get(hello))
        .route("/users/{user}/tasks/{task}", get(user_task))
        .route("/assets/{*rest}", get(static_like))
        .fallback(not_found)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app())
        .with_graceful_shutdown(shutdown_signal())
        .await
}
