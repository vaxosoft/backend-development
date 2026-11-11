//! Request ID, access log and panic recovery.

use std::any::Any;
use std::time::Instant;

use axum::body::Body;
use axum::extract::{Request, State};
use axum::http::{HeaderName, HeaderValue, StatusCode};
use axum::middleware::Next;
use axum::response::Response;
use uuid::Uuid;

use crate::LogSink;
use crate::error::error_response;

pub const REQUEST_ID_HEADER: HeaderName = HeaderName::from_static("x-request-id");

const MAX_REQUEST_ID_LEN: usize = 128;

fn client_request_id(req: &Request) -> Option<HeaderValue> {
    let value = req.headers().get(&REQUEST_ID_HEADER)?;
    (!value.is_empty() && value.len() <= MAX_REQUEST_ID_LEN).then(|| value.clone())
}

pub async fn request_id(mut req: Request, next: Next) -> Response {
    let id = client_request_id(&req).unwrap_or_else(|| {
        HeaderValue::from_str(&Uuid::new_v4().to_string()).expect("a UUID is a valid header value")
    });
    req.headers_mut().insert(REQUEST_ID_HEADER, id.clone());
    let mut response = next.run(req).await;
    response.headers_mut().insert(REQUEST_ID_HEADER, id);
    response
}

pub async fn access_log(State(sink): State<LogSink>, req: Request, next: Next) -> Response {
    let method = req.method().clone();
    let path = req.uri().path().to_owned();
    let id = req
        .headers()
        .get(&REQUEST_ID_HEADER)
        .and_then(|v| v.to_str().ok())
        .unwrap_or("-")
        .to_owned();
    let started = Instant::now();

    let response = next.run(req).await;

    sink(&format!(
        "method={method} path={path} status={} duration_ms={} request_id={id}",
        response.status().as_u16(),
        started.elapsed().as_millis(),
    ));
    response
}

/// The payload is only useful for server-side logging; clients get a fixed body.
pub fn panic_response(_payload: Box<dyn Any + Send + 'static>) -> axum::http::Response<Body> {
    error_response(
        StatusCode::INTERNAL_SERVER_ERROR,
        "internal",
        "internal server error",
    )
}
