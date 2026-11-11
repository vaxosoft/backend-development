//! Fan-out over many ids.

use crate::{AggregateResponse, AppState};

/// Fetch every id from the upstream and merge the outcomes.
///
/// TODO:
/// - run the fetches concurrently, but never more than `config.max_concurrency` at once
/// - keep request order in `results` and in `errors`
/// - a failed id becomes `Failure { id, error: err.to_string() }`; the response is still a success
/// - `config.overall_timeout` is one deadline for the whole request: ids that are
///   not finished by then fail with "deadline exceeded" and their upstream calls are dropped
pub async fn run(_state: &AppState, _ids: Vec<u64>) -> AggregateResponse {
    todo!("fan out with bounded concurrency and an overall deadline")
}
