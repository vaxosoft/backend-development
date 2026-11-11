//! Exercise 2: retry idempotent GETs with exponential backoff.
use std::time::Duration;

use reqwest::{Client, StatusCode};

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
    pub fn delay(&self, retry: u32) -> Duration {
        self.base
            .saturating_mul(2u32.saturating_pow(retry))
            .min(self.cap)
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
    client: &Client,
    url: &str,
    policy: &RetryPolicy,
) -> Result<String, RetryError> {
    let mut attempts = 0;
    loop {
        attempts += 1;
        let failure = match client.get(url).send().await {
            Ok(response) if response.status().is_success() => match response.text().await {
                Ok(body) => return Ok(body),
                Err(error) => error.to_string(),
            },
            Ok(response)
                if response.status().is_server_error()
                    || response.status() == StatusCode::TOO_MANY_REQUESTS =>
            {
                format!("status {}", response.status().as_u16())
            }
            Ok(response) => return Err(RetryError::Permanent(response.status().as_u16())),
            Err(error) => error.to_string(),
        };
        if attempts > policy.max_retries {
            return Err(RetryError::Exhausted {
                attempts,
                last: failure,
            });
        }
        tokio::time::sleep(policy.delay(attempts - 1)).await;
    }
}
