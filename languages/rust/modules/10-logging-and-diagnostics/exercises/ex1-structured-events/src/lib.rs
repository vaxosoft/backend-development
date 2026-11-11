//! Exercise 1: emit well-formed structured events and build a log filter safely.
use std::time::Duration;

use tracing_subscriber::EnvFilter;

#[derive(Debug, Clone)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: String,
}

/// INFO, message `"task created"`, with the fields `task_id` (a number, not a string) and
/// `status`. The title may contain personal data, so it goes into a SEPARATE DEBUG event with
/// the message `"task details"` and the field `title`.
pub fn log_task_created(_task: &Task) {
    todo!("one info! event and one debug! event")
}

/// ERROR, message `"store operation failed"`, with the fields `task_id` (number) and `error`
/// (the error's `Display` text, recorded as a string).
pub fn log_store_error(_task_id: u64, _error: &dyn std::error::Error) {
    todo!("one error! event")
}

/// Slow operations deserve attention, fast ones do not:
/// - `elapsed > threshold`: WARN, message `"slow operation"`
/// - otherwise: DEBUG, message `"operation finished"`
///
/// Both carry the fields `op` (string), `elapsed_ms` (number) and `threshold_ms` (number).
pub fn log_operation(_op: &str, _elapsed: Duration, _threshold: Duration) {
    todo!("pick the level from the comparison, record durations as milliseconds")
}

/// Build the log filter from the value of `RUST_LOG` (`None` when it is not set).
/// A missing OR invalid value falls back to `default_directives`; starting a server must not
/// fail because of a typo in an environment variable.
pub fn build_filter(_rust_log: Option<&str>, _default_directives: &str) -> EnvFilter {
    todo!("EnvFilter::try_new with a fallback")
}
