//! Fan-out over many ids.

use futures::StreamExt;
use tokio::time::{Instant, timeout_at};

use crate::upstream::fetch_with_retry;
use crate::{AggregateResponse, AppState, Failure, Success};

/// Fetch every id from the upstream and merge the outcomes.
pub async fn run(state: &AppState, ids: Vec<u64>) -> AggregateResponse {
    let config = &state.config;
    let deadline = Instant::now() + config.overall_timeout;

    // `buffered` polls at most N futures at a time and yields their output in input order.
    // Dropping the stream (client gone) or the `timeout_at` future drops the in-flight requests.
    let outcomes: Vec<_> = futures::stream::iter(ids)
        .map(|id| async move {
            let call = fetch_with_retry(&state.client, config, id);
            let outcome = match timeout_at(deadline, call).await {
                Ok(result) => result.map_err(|e| e.to_string()),
                Err(_elapsed) => Err("deadline exceeded".to_string()),
            };
            (id, outcome)
        })
        .buffered(config.max_concurrency.max(1))
        .collect()
        .await;

    let mut response = AggregateResponse::default();
    for (id, outcome) in outcomes {
        match outcome {
            Ok(data) => response.results.push(Success { id, data }),
            Err(error) => response.errors.push(Failure { id, error }),
        }
    }
    response
}
