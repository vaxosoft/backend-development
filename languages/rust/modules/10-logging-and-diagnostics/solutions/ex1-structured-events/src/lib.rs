//! Exercise 1: emit well-formed structured events and build a log filter safely.
use std::time::Duration;

use tracing::{debug, error, info, warn};
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
pub fn log_task_created(task: &Task) {
    info!(task_id = task.id, status = %task.status, "task created");
    debug!(task_id = task.id, title = %task.title, "task details");
}

/// ERROR, message `"store operation failed"`, with the fields `task_id` (number) and `error`
/// (the error's `Display` text, recorded as a string).
pub fn log_store_error(task_id: u64, error: &dyn std::error::Error) {
    error!(task_id, error = %error, "store operation failed");
}

/// Slow operations deserve attention, fast ones do not:
/// - `elapsed > threshold`: WARN, message `"slow operation"`
/// - otherwise: DEBUG, message `"operation finished"`
///
/// Both carry the fields `op` (string), `elapsed_ms` (number) and `threshold_ms` (number).
pub fn log_operation(op: &str, elapsed: Duration, threshold: Duration) {
    let elapsed_ms = elapsed.as_millis() as u64;
    let threshold_ms = threshold.as_millis() as u64;
    if elapsed > threshold {
        warn!(op, elapsed_ms, threshold_ms, "slow operation");
    } else {
        debug!(op, elapsed_ms, threshold_ms, "operation finished");
    }
}

/// Build the log filter from the value of `RUST_LOG` (`None` when it is not set).
/// A missing OR invalid value falls back to `default_directives`; starting a server must not
/// fail because of a typo in an environment variable.
pub fn build_filter(rust_log: Option<&str>, default_directives: &str) -> EnvFilter {
    rust_log
        .and_then(|directives| EnvFilter::try_new(directives).ok())
        .unwrap_or_else(|| EnvFilter::new(default_directives))
}
