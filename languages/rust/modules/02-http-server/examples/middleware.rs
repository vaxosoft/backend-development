//! Middleware: `from_fn`, layer ordering, timeouts, body limits and CORS.
//!
//! ```text
//! ADDR=127.0.0.1:20403 cargo run -p m02-http-server --example middleware
//! ```

use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};

use axum::Router;
use axum::body::Bytes;
use axum::extract::{DefaultBodyLimit, Query, Request, State};
use axum::http::{HeaderMap, HeaderName, HeaderValue, Method, StatusCode, header};
use axum::middleware::{self, Next};
use axum::response::Response;
use axum::routing::{get, post};
use m02_http_server::{bind_addr, shutdown_signal};
use serde::Deserialize;
use tokio::net::TcpListener;
use tower_http::cors::CorsLayer;
use tower_http::timeout::TimeoutLayer;

static REQUEST_ID: HeaderName = HeaderName::from_static("x-request-id");

#[derive(Clone, Default)]
struct Ids(Arc<AtomicU64>);

// A middleware is an async fn that gets the request and `Next`. Code before
// `next.run` sees the request; code after it sees the response.
async fn request_id(State(ids): State<Ids>, mut req: Request, next: Next) -> Response {
    let id = req.headers().get(&REQUEST_ID).cloned().unwrap_or_else(|| {
        let n = ids.0.fetch_add(1, Ordering::Relaxed) + 1;
        HeaderValue::from_str(&format!("req-{n}")).expect("ascii")
    });
    req.headers_mut().insert(REQUEST_ID.clone(), id.clone());

    let started = Instant::now();
    let method = req.method().clone();
    let path = req.uri().path().to_string();
    let mut response = next.run(req).await;
    eprintln!(
        "{method} {path} -> {} in {:?}",
        response.status().as_u16(),
        started.elapsed()
    );
    response.headers_mut().insert(REQUEST_ID.clone(), id);
    response
}

// Appends its name to a request header so the handler can show the order.
async fn tag(name: &'static str, mut req: Request, next: Next) -> Response {
    let seen = req
        .headers()
        .get("x-seen")
        .and_then(|v| v.to_str().ok())
        .map_or(name.to_string(), |prev| format!("{prev},{name}"));
    req.headers_mut()
        .insert("x-seen", HeaderValue::from_str(&seen).expect("ascii"));
    next.run(req).await
}

async fn order(headers: HeaderMap) -> String {
    let seen = headers.get("x-seen").and_then(|v| v.to_str().ok());
    format!("{}\n", seen.unwrap_or(""))
}

#[derive(Deserialize)]
struct Delay {
    ms: u64,
}

async fn slow(Query(delay): Query<Delay>) -> &'static str {
    tokio::time::sleep(Duration::from_millis(delay.ms)).await;
    "done\n"
}

async fn upload(body: Bytes) -> String {
    format!("received {} bytes\n", body.len())
}

fn app() -> Router {
    let cors = CorsLayer::new()
        .allow_origin(HeaderValue::from_static("https://app.example"))
        .allow_methods([Method::GET, Method::POST])
        .allow_headers([header::CONTENT_TYPE]);

    Router::new()
        .route("/order", get(order))
        .route("/slow", get(slow))
        // Routes added before a `.layer(...)` call are wrapped by it; the Bytes
        // extractor honours this 16-byte limit (axum's default is 2 MiB).
        .route("/upload", post(upload).layer(DefaultBodyLimit::max(16)))
        // The LAST layer added is the OUTERMOST: it runs first on the way in.
        .layer(middleware::from_fn(|req, next| tag("inner", req, next)))
        .layer(middleware::from_fn(|req, next| tag("outer", req, next)))
        .layer(TimeoutLayer::with_status_code(
            StatusCode::REQUEST_TIMEOUT,
            Duration::from_millis(200),
        ))
        .layer(cors)
        .layer(middleware::from_fn_with_state(Ids::default(), request_id))
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    let listener = TcpListener::bind(bind_addr()).await?;
    eprintln!("listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app())
        .with_graceful_shutdown(shutdown_signal())
        .await
}
