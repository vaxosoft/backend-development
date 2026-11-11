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
    pub fn delay_for(&self, _retry: u32) -> Duration {
        todo!("exponential backoff with a cap; no overflow panics")
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
    _policy: &RetryPolicy,
    _sleeper: &S,
    _op: F,
) -> Result<T, RetryError<E>>
where
    S: Sleeper,
    F: FnMut(u32) -> Fut,
    Fut: Future<Output = Result<T, Attempt<E>>>,
{
    todo!("loop over attempts, sleeping policy.delay_for(n) between them")
}
