//! Kept in its own test binary on purpose: tracing caches, per call site, whether anyone is
//! interested, so a capture can come back empty when other tests in the process log first.
use std::io;
use std::sync::{Arc, Mutex};

use axum::{body::Body, http::Request};
use m10_ex3_diagnostics_router::{Metrics, build_app};
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

#[tokio::test]
async fn the_panic_message_is_logged_as_an_error_event() {
    let capture = Capture::default();
    let writer = capture.clone();
    let subscriber = tracing_subscriber::fmt()
        .json()
        .with_writer(move || writer.clone())
        .finish();
    let _guard = tracing::subscriber::set_default(subscriber);

    let request = Request::builder().uri("/boom").body(Body::empty()).unwrap();
    build_app(Metrics::default())
        .oneshot(request)
        .await
        .unwrap();

    let text = String::from_utf8(capture.0.lock().unwrap().clone()).unwrap();
    let record = text
        .lines()
        .map(|line| serde_json::from_str::<serde_json::Value>(line).unwrap())
        .find(|r| r["fields"]["message"] == "handler panicked")
        .unwrap_or_else(|| panic!("no 'handler panicked' event in: {text}"));
    assert_eq!(record["level"], "ERROR");
    assert_eq!(record["fields"]["panic"], "kaboom");
}
