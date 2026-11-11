//! Writing responses: status codes, headers, content negotiation, cookies,
//! streaming bodies and static files.
//!
//! ```text
//! ADDR=127.0.0.1:20402 cargo run -p m02-http-server --example responses
//! ```

use std::convert::Infallible;
use std::time::Duration;

use axum::body::{Body, Bytes};
use axum::http::{HeaderMap, StatusCode, header};
use axum::response::{IntoResponse, Redirect, Response};
use axum::routing::get;
use axum::{Json, Router};
use axum_extra::extract::cookie::{Cookie, CookieJar};
use futures::StreamExt;
use m02_http_server::{bind_addr, shutdown_signal};
use serde_json::json;
use tokio::net::TcpListener;
use tower_http::services::ServeDir;

// A tuple of (status, headers, body) implements `IntoResponse`.
async fn created() -> impl IntoResponse {
    (
        StatusCode::CREATED,
        [(header::LOCATION, "/tasks/1")],
        Json(json!({ "id": 1, "title": "Write docs" })),
    )
}

async fn old_path() -> Redirect {
    Redirect::permanent("/new-path")
}

async fn no_content() -> StatusCode {
    StatusCode::NO_CONTENT
}

// Hand-rolled negotiation: pick a representation from the `Accept` header.
async fn negotiate(headers: HeaderMap) -> Response {
    let accept = headers
        .get(header::ACCEPT)
        .and_then(|v| v.to_str().ok())
        .unwrap_or("*/*");
    if accept.contains("application/json") || accept.contains("*/*") {
        Json(json!({ "message": "hello" })).into_response()
    } else if accept.contains("text/plain") {
        "hello\n".into_response()
    } else {
        (
            StatusCode::NOT_ACCEPTABLE,
            "supported: application/json, text/plain\n",
        )
            .into_response()
    }
}

// `CookieJar` is both an extractor and a response part: return it to emit `Set-Cookie`.
async fn set_theme(jar: CookieJar) -> (CookieJar, &'static str) {
    let cookie = Cookie::build(("theme", "dark"))
        .path("/")
        .http_only(true)
        .build();
    (jar.add(cookie), "theme saved\n")
}

async fn get_theme(jar: CookieJar) -> String {
    let theme = jar.get("theme").map_or("none", |c| c.value());
    format!("theme: {theme}\n")
}

// The body is produced chunk by chunk; the response has no Content-Length
// and is sent with chunked transfer encoding.
async fn countdown() -> Response {
    let chunks = futures::stream::iter(1..=3).then(|n| async move {
        tokio::time::sleep(Duration::from_millis(20)).await;
        Ok::<_, Infallible>(Bytes::from(format!("tick {n}\n")))
    });
    Response::builder()
        .header(header::CONTENT_TYPE, "text/plain; charset=utf-8")
        .body(Body::from_stream(chunks))
        .expect("static headers are valid")
}

fn app() -> Router {
    let static_dir = concat!(env!("CARGO_MANIFEST_DIR"), "/examples/static");
    Router::new()
        .route("/created", get(created))
        .route("/old-path", get(old_path))
        .route("/no-content", get(no_content))
        .route("/negotiate", get(negotiate))
        .route("/theme", get(get_theme).post(set_theme))
        .route("/countdown", get(countdown))
        .nest_service("/static", ServeDir::new(static_dir))
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app())
        .with_graceful_shutdown(shutdown_signal())
        .await
}
