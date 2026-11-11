//! JSON logs, and capturing log output in memory (the technique tests use).
use std::io;
use std::sync::{Arc, Mutex};

use serde_json::Value;
use tracing::{info, info_span, warn};

#[derive(Clone, Default)]
struct Capture(Arc<Mutex<Vec<u8>>>);

impl io::Write for Capture {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        self.0.lock().expect("lock").extend_from_slice(buf);
        Ok(buf.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

fn main() {
    let capture = Capture::default();
    let writer = capture.clone();
    let subscriber = tracing_subscriber::fmt()
        .json()
        .without_time()
        .with_current_span(true)
        .with_span_list(false)
        // `MakeWriter` is implemented for closures that return a writer.
        .with_writer(move || writer.clone())
        .finish();

    tracing::subscriber::with_default(subscriber, || {
        info!(task_id = 42, status = "todo", "task created");
        let span = info_span!("request", request_id = "req-0001", user_id = 7);
        span.in_scope(|| warn!(retries = 2, "slow query"));
    });

    let output = String::from_utf8(capture.0.lock().expect("lock").clone()).expect("utf-8 logs");
    println!("raw output:\n{output}");

    // One JSON object per line: easy to parse, ship and query.
    for line in output.lines() {
        let record: Value = serde_json::from_str(line).expect("each line is JSON");
        println!(
            "level={} message={} task_id={} span={}",
            record["level"],
            record["fields"]["message"],
            record["fields"]["task_id"],
            record["span"]["request_id"]
        );
    }
}
