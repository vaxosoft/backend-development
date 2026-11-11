use std::io;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use m10_ex1_structured_events::{
    Task, build_filter, log_operation, log_store_error, log_task_created,
};
use serde_json::Value;
use tracing::level_filters::LevelFilter;

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

/// Run `f` with a JSON subscriber at `max_level` and return every record it logged.
fn capture(max_level: tracing::Level, f: impl FnOnce()) -> Vec<Value> {
    let capture = Capture::default();
    let writer = capture.clone();
    let subscriber = tracing_subscriber::fmt()
        .json()
        .with_max_level(max_level)
        .with_writer(move || writer.clone())
        .finish();
    tracing::subscriber::with_default(subscriber, f);
    let text = String::from_utf8(capture.0.lock().unwrap().clone()).unwrap();
    text.lines()
        .map(|line| serde_json::from_str(line).unwrap())
        .collect()
}

fn task() -> Task {
    Task {
        id: 42,
        title: "Call Alice about her tax return".into(),
        status: "todo".into(),
    }
}

#[test]
fn task_created_is_an_info_event_with_typed_fields() {
    let records = capture(tracing::Level::INFO, || log_task_created(&task()));

    assert_eq!(
        records.len(),
        1,
        "the DEBUG details must be filtered out at INFO: {records:?}"
    );
    assert_eq!(records[0]["level"], "INFO");
    assert_eq!(records[0]["fields"]["message"], "task created");
    assert_eq!(
        records[0]["fields"]["task_id"], 42,
        "task_id must be a JSON number"
    );
    assert_eq!(records[0]["fields"]["status"], "todo");
}

#[test]
fn the_title_is_only_logged_at_debug() {
    let at_info = capture(tracing::Level::INFO, || log_task_created(&task()));
    assert!(
        !at_info[0].to_string().contains("tax return"),
        "title leaked at INFO: {}",
        at_info[0]
    );

    let at_debug = capture(tracing::Level::DEBUG, || log_task_created(&task()));
    assert_eq!(at_debug.len(), 2);
    assert_eq!(at_debug[1]["level"], "DEBUG");
    assert_eq!(at_debug[1]["fields"]["message"], "task details");
    assert_eq!(
        at_debug[1]["fields"]["title"],
        "Call Alice about her tax return"
    );
}

#[test]
fn store_errors_log_the_display_text_as_a_string() {
    let error = io::Error::new(io::ErrorKind::ConnectionReset, "connection reset by peer");
    let records = capture(tracing::Level::INFO, || log_store_error(7, &error));

    assert_eq!(records.len(), 1);
    assert_eq!(records[0]["level"], "ERROR");
    assert_eq!(records[0]["fields"]["message"], "store operation failed");
    assert_eq!(records[0]["fields"]["task_id"], 7);
    assert_eq!(records[0]["fields"]["error"], "connection reset by peer");
}

#[test]
fn slow_operations_warn_with_millisecond_numbers() {
    let records = capture(tracing::Level::INFO, || {
        log_operation(
            "load_task",
            Duration::from_millis(250),
            Duration::from_millis(100),
        );
    });

    assert_eq!(records.len(), 1);
    assert_eq!(records[0]["level"], "WARN");
    assert_eq!(records[0]["fields"]["message"], "slow operation");
    assert_eq!(records[0]["fields"]["op"], "load_task");
    assert_eq!(records[0]["fields"]["elapsed_ms"], 250);
    assert_eq!(records[0]["fields"]["threshold_ms"], 100);
}

#[test]
fn fast_operations_only_show_up_at_debug_and_exactly_at_the_threshold_is_fast() {
    let fast = || {
        log_operation(
            "load_task",
            Duration::from_millis(100),
            Duration::from_millis(100),
        )
    };
    assert!(capture(tracing::Level::INFO, fast).is_empty());

    let records = capture(tracing::Level::DEBUG, fast);
    assert_eq!(records.len(), 1);
    assert_eq!(records[0]["level"], "DEBUG");
    assert_eq!(records[0]["fields"]["message"], "operation finished");
}

#[test]
fn the_filter_uses_rust_log_when_it_is_valid() {
    assert_eq!(
        build_filter(Some("debug"), "warn").max_level_hint(),
        Some(LevelFilter::DEBUG)
    );
    assert_eq!(
        build_filter(Some("error,tasks=trace"), "warn").max_level_hint(),
        Some(LevelFilter::TRACE)
    );
}

#[test]
fn the_filter_falls_back_to_the_default_when_missing_or_invalid() {
    assert_eq!(
        build_filter(None, "warn").max_level_hint(),
        Some(LevelFilter::WARN)
    );
    assert_eq!(
        build_filter(Some("tasks=loud"), "info").max_level_hint(),
        Some(LevelFilter::INFO)
    );
    assert_eq!(
        build_filter(Some("=[[="), "error").max_level_hint(),
        Some(LevelFilter::ERROR)
    );
}
