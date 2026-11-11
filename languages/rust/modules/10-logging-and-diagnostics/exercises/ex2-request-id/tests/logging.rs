//! Kept in its own test binary on purpose: tracing caches, per call site, whether anyone is
//! interested. If other tests in the same process hit the same `info!` without a subscriber
//! first, this capture can come back empty.
use std::io;
use std::sync::{Arc, Mutex};

use axum::{Extension, Router, body::Body, http::Request, routing::get};
use m10_ex2_request_id::{IdGenerator, RequestId, with_request_ids};
use serde_json::Value;
use tower::ServiceExt;

#[derive(Clone, Default)]
struct Capture(Arc<Mutex<Vec<u8>>>);

impl io::Write for Capture {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.0.lock().unwrap().extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

async fn handler(Extension(id): Extension<RequestId>) -> String {
    tracing::info!("inside the handler");
    id.0
}

#[tokio::test]
async fn events_logged_in_the_handler_carry_the_request_id() {
    let capture = Capture::default();
    let writer = capture.clone();
    let subscriber = tracing_subscriber::fmt()
        .json()
        .with_current_span(true)
        .with_writer(move || writer.clone())
        .finish();
    let _guard = tracing::subscriber::set_default(subscriber);

    let app: Router = with_request_ids(
        Router::new().route("/whoami", get(handler)),
        IdGenerator::default(),
    );
    for header in [Some("demo-7"), None] {
        let mut request = Request::builder().uri("/whoami");
        if let Some(value) = header {
            request = request.header("x-request-id", value);
        }
        app.clone()
            .oneshot(request.body(Body::empty()).unwrap())
            .await
            .unwrap();
    }

    let text = String::from_utf8(capture.0.lock().unwrap().clone()).unwrap();
    let records: Vec<Value> = text
        .lines()
        .map(|l| serde_json::from_str(l).unwrap())
        .collect();
    let ids: Vec<&str> = records
        .iter()
        .filter(|r| r["fields"]["message"] == "inside the handler")
        .map(|r| r["span"]["request_id"].as_str().unwrap())
        .collect();
    assert_eq!(ids, ["demo-7", "req-0001"]);
}
