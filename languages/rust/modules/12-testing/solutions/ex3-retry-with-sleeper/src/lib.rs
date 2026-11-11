//! Exercise 3: retry with exponential backoff, driven by an injectable sleeper so tests
//! never wait and can assert the exact delays.

use std::future::Future;
use std::time::Duration;

/// Abstraction over "wait for a while". Tests substitute a recorder that returns immediately.
#[allow(async_fn_in_trait)] // only used with concrete types, never as `dyn Sleeper`
pub trait Sleeper {
    async fn sleep(&self, duration: Duration);
}

pub struct TokioSleeper;

impl Sleeper for TokioSleeper {
    async fn sleep(&self, duration: Duration) {
        tokio::time::sleep(duration).await;
    }
}

#[derive(Clone, Debug)]
pub struct RetryPolicy {
    /// Total attempts including the first one (0 is treated as 1).
    pub max_attempts: u32,
    pub base_delay: Duration,
    pub max_delay: Duration,
}

impl RetryPolicy {
    /// Delay before retry number `retry` (1 = the wait after the first failure):
    /// `base * 2^(retry-1)`, capped at `max_delay`, and safe for huge `retry` values.
    pub fn delay_for(&self, retry: u32) -> Duration {
        let doublings = retry.saturating_sub(1).min(32);
        self.base_delay
            .saturating_mul(1u32 << doublings.min(31))
            .min(self.max_delay)
    }
}

/// What a single attempt reports on failure.
#[derive(Debug, PartialEq, Eq)]
pub enum Attempt<E> {
    /// Worth trying again (timeout, 503, connection reset...).
    Retryable(E),
    /// Retrying cannot help (400, 404, validation...).
    Fatal(E),
}

#[derive(Debug, PartialEq, Eq)]
pub enum RetryError<E> {
    /// Ran out of attempts; carries the last error.
    Exhausted { attempts: u32, last: E },
    /// A fatal error stopped the loop early.
    Fatal { attempts: u32, error: E },
}

/// Calls `op(attempt_number)` (starting at 1) until it succeeds, fails fatally, or the policy is
/// exhausted. Sleeps between attempts only, never after the last one.
pub async fn retry<T, E, S, F, Fut>(
    policy: &RetryPolicy,
    sleeper: &S,
    mut op: F,
) -> Result<T, RetryError<E>>
where
    S: Sleeper,
    F: FnMut(u32) -> Fut,
    Fut: Future<Output = Result<T, Attempt<E>>>,
{
    let max_attempts = policy.max_attempts.max(1);
    let mut attempt = 1;
    loop {
        match op(attempt).await {
            Ok(value) => return Ok(value),
            Err(Attempt::Fatal(error)) => {
                return Err(RetryError::Fatal {
                    attempts: attempt,
                    error,
                });
            }
            Err(Attempt::Retryable(last)) => {
                if attempt >= max_attempts {
                    return Err(RetryError::Exhausted {
                        attempts: attempt,
                        last,
                    });
                }
                sleeper.sleep(policy.delay_for(attempt)).await;
                attempt += 1;
            }
        }
    }
}
