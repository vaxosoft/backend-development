//! What each job type does.

use crate::jobs::JobSpec;

/// Runs one attempt of a job and returns its result or an error message.
///
/// * `reverse`: the text reversed (by `char`)
/// * `sleep`: wait `ms` milliseconds without blocking the thread, result `"slept {ms}ms"`
/// * `fail`: always `Err`
///
/// Cancellation and timeouts are not handled here: the pool simply drops this
/// future when it must stop, which cancels it at its next `.await`.
pub async fn execute(_spec: &JobSpec) -> Result<String, String> {
    todo!("TODO: implement the three job types")
}
