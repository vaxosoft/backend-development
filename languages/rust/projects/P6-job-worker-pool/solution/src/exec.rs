//! What each job type does.

use std::time::Duration;

use crate::jobs::JobSpec;

/// Runs one attempt of a job and returns its result or an error message.
///
/// Cancellation and timeouts are not handled here: the pool simply drops this
/// future when it must stop, which cancels it at its next `.await`.
pub async fn execute(spec: &JobSpec) -> Result<String, String> {
    match spec {
        JobSpec::Reverse { text } => Ok(text.chars().rev().collect()),
        JobSpec::Sleep { ms } => {
            tokio::time::sleep(Duration::from_millis(*ms)).await;
            Ok(format!("slept {ms}ms"))
        }
        JobSpec::Fail => Err("job always fails".to_owned()),
    }
}
