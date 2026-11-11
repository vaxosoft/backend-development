//! One upstream item, fetched with a timeout and retries.

use serde_json::Value;

use crate::Config;

/// Why an upstream call failed. `Display` is the text clients see in `errors[].error`.
#[derive(Debug, thiserror::Error)]
pub enum FetchError {
    #[error("upstream status {0}")]
    Status(u16),
    #[error("upstream timeout")]
    Timeout,
    #[error("upstream unreachable")]
    Network,
    #[error("invalid upstream response")]
    BadBody,
}

/// GET `{upstream_base}/items/{id}` and return its JSON body.
///
/// TODO:
/// - every attempt is limited to `config.call_timeout`
/// - retry up to `config.max_retries` times on network errors, timeouts and 5xx
///   statuses, sleeping `backoff_base`, then twice that, then four times, ...
/// - never retry a 4xx (or a body that is not JSON)
pub async fn fetch_with_retry(
    _client: &reqwest::Client,
    _config: &Config,
    _id: u64,
) -> Result<Value, FetchError> {
    todo!("call the upstream with timeout, retry and backoff")
}
