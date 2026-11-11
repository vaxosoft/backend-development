//! Events, levels, structured fields and filter directives.
//! The same `emit()` is run under three subscribers to show what a filter lets through.
use tracing::{debug, error, info, trace, warn};
use tracing_subscriber::EnvFilter;

#[derive(Debug)]
struct Task {
    id: u64,
    title: &'static str,
}

fn emit() {
    let task = Task {
        id: 42,
        title: "Write docs",
    };
    trace!("very chatty detail");
    debug!(target: "tasks::store", sql = "SELECT * FROM tasks WHERE id = ?", "executing query");
    debug!(task_id = task.id, "loading task");
    // `field = value` records a typed field; `?` uses Debug, `%` uses Display.
    info!(task_id = task.id, status = "todo", "task created");
    info!(task = ?task, title = %task.title, "full record");
    warn!(target: "tasks::store", retries = 2, "slow query");
    error!(target: "tasks::store", error = %"connection reset", "query failed");
}

fn with_directives(directives: &str) {
    println!("--- RUST_LOG={directives}");
    let subscriber = tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::new(directives))
        .without_time() // timestamps are off only so this lesson's output is stable
        .with_ansi(false)
        .with_writer(std::io::stdout)
        .finish();
    // `with_default` scopes the subscriber to this closure; a real program installs one
    // globally, once, at startup (see example 03 and 04).
    tracing::subscriber::with_default(subscriber, emit);
}

fn main() {
    with_directives("info");
    with_directives("debug");
    // Per-target directives: everything at `warn`, but the `tasks` modules at `trace`.
    with_directives("warn,tasks=trace");
}
