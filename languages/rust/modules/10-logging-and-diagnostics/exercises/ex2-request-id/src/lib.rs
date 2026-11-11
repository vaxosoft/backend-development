//! Exercise 2: a request-id middleware that also puts the id on every log line of the request.
use std::sync::Arc;
use std::sync::atomic::AtomicU64;

use axum::{
    Router,
    extract::{Request, State},
    middleware::{self, Next},
    response::Response,
};

/// Handlers can read the id with `Extension<RequestId>`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RequestId(pub String);

/// Hands out `req-0001`, `req-0002`, ... (4 digits, zero padded, starting at 1).
/// Clones share one counter.
#[derive(Clone, Default)]
pub struct IdGenerator(#[allow(dead_code)] Arc<AtomicU64>);

impl IdGenerator {
    pub fn next_id(&self) -> String {
        todo!("increment the shared counter and format it")
    }
}

/// An incoming id is only trusted if it is 1 to 64 characters, all printable ASCII without
/// spaces (`char::is_ascii_graphic`). Anything else could be used to forge or flood log lines.
pub fn is_valid_request_id(_value: &str) -> bool {
    todo!("length and character check")
}

/// For each request:
/// 1. take the `x-request-id` header if it is valid, otherwise generate an id
/// 2. store a [`RequestId`] in the request extensions
/// 3. run the rest of the stack inside an `info_span!("request", request_id = %id)` so every
///    event logged by handlers carries the id (attach the span with `Instrument`, do not hold
///    `span.enter()` across an `.await`)
/// 4. set `x-request-id` on the response
pub async fn request_id_middleware(
    State(_ids): State<IdGenerator>,
    _request: Request,
    _next: Next,
) -> Response {
    todo!("see the steps above")
}

pub fn with_request_ids(router: Router, ids: IdGenerator) -> Router {
    router.layer(middleware::from_fn_with_state(ids, request_id_middleware))
}
