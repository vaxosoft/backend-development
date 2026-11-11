//! Request ID, access log and panic recovery. All TODO stubs.

use std::any::Any;

use axum::body::Body;
use axum::extract::{Request, State};
use axum::http::{HeaderName, StatusCode};
use axum::middleware::Next;
use axum::response::Response;

use crate::LogSink;
use crate::error::error_response;

pub const REQUEST_ID_HEADER: HeaderName = HeaderName::from_static("x-request-id");

/// TODO: echo a client-supplied `X-Request-ID` (non-empty, at most 128 bytes)
/// or generate a UUID. Make the id visible to later layers (request headers)
/// and set it on the response.
pub async fn request_id(req: Request, next: Next) -> Response {
    next.run(req).await
}

/// TODO: after the inner service answers, send ONE line to the sink:
/// `method=GET path=/healthz status=200 duration_ms=3 request_id=<id>`.
pub async fn access_log(State(_sink): State<LogSink>, req: Request, next: Next) -> Response {
    next.run(req).await
}

/// TODO: turn a caught panic into a 500 `internal` envelope.
/// Never include the panic message in the response.
pub fn panic_response(_payload: Box<dyn Any + Send + 'static>) -> axum::http::Response<Body> {
    error_response(StatusCode::NOT_IMPLEMENTED, "todo", "panic_response")
}
