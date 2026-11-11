//! Exercise 2: a keyed token-bucket rate limiter with an injected clock.

use std::collections::HashMap;
use std::time::{Duration, Instant};

#[derive(Debug, PartialEq)]
pub struct Denied {
    pub retry_after: Duration,
}

impl Denied {
    /// Value for the `Retry-After` header: whole seconds, rounded up, never 0.
    pub fn retry_after_secs(&self) -> u64 {
        todo!("round retry_after up to whole seconds, minimum 1")
    }
}

#[allow(dead_code)] // the fields are used by your implementation
struct Bucket {
    tokens: f64,
    last: Instant,
}

/// One bucket per key (for example a user id). A new key starts full.
pub struct RateLimiter {
    #[allow(dead_code)]
    burst: f64,
    #[allow(dead_code)]
    refill_per_sec: f64,
    buckets: HashMap<String, Bucket>,
}

impl RateLimiter {
    pub fn new(burst: u32, refill_per_sec: f64) -> Self {
        Self {
            burst: f64::from(burst),
            refill_per_sec,
            buckets: HashMap::new(),
        }
    }

    /// Takes one token for `key`. `now` is passed in so tests control time.
    pub fn check(&mut self, _key: &str, _now: Instant) -> Result<(), Denied> {
        todo!("refill by elapsed time (capped at burst), then take a token or report the wait")
    }

    /// Drops buckets that have been idle long enough to be full again; returns how many.
    pub fn evict_idle(&mut self, _now: Instant) -> usize {
        todo!("remove buckets that would be back at full capacity")
    }

    pub fn tracked_keys(&self) -> usize {
        self.buckets.len()
    }
}
