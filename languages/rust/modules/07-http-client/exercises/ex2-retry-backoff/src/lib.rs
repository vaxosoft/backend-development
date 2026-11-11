//! Exercise 2: retry idempotent GETs with exponential backoff.
use std::time::Duration;

use reqwest::Client;

#[derive(Debug, Clone, Copy)]
pub struct RetryPolicy {
    /// Retries after the first attempt, so a request is sent at most `max_retries + 1` times.
    pub max_retries: u32,
    pub base: Duration,
    pub cap: Duration,
}

impl RetryPolicy {
    /// Delay before retry number `retry` (0 is the first retry): `base * 2^retry`, never above
    /// `cap`. Must not panic or overflow for huge `retry` values.
    pub fn delay(&self, _retry: u32) -> Duration {
        todo!("exponential backoff with a cap")
    }
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum RetryError {
    /// A non-retryable status (4xx other than 429): returned immediately.
    #[error("permanent failure: status {0}")]
    Permanent(u16),
    /// Every attempt failed with a retryable problem.
    #[error("gave up after {attempts} attempts: {last}")]
    Exhausted { attempts: u32, last: String },
}

/// GET `url` and return the body text.
///
/// Retry (sleeping `policy.delay(n)` between attempts) on connection errors, timeouts,
/// 5xx and 429. Do not retry any other non-2xx status.
pub async fn get_text(
    _client: &Client,
    _url: &str,
    _policy: &RetryPolicy,
) -> Result<String, RetryError> {
    todo!("loop: send, classify the outcome, sleep, repeat")
}
