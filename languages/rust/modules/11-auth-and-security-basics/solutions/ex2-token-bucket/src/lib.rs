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
        (self.retry_after.as_secs_f64().ceil() as u64).max(1)
    }
}

struct Bucket {
    tokens: f64,
    last: Instant,
}

/// One bucket per key (for example a user id). A new key starts full.
pub struct RateLimiter {
    burst: f64,
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
    pub fn check(&mut self, key: &str, now: Instant) -> Result<(), Denied> {
        let bucket = self.buckets.entry(key.to_string()).or_insert(Bucket {
            tokens: self.burst,
            last: now,
        });
        // saturating: a clock that goes backwards must not mint tokens or panic
        let elapsed = now.saturating_duration_since(bucket.last).as_secs_f64();
        bucket.tokens = (bucket.tokens + elapsed * self.refill_per_sec).min(self.burst);
        bucket.last = bucket.last.max(now);

        if bucket.tokens >= 1.0 {
            bucket.tokens -= 1.0;
            Ok(())
        } else {
            let missing = 1.0 - bucket.tokens;
            Err(Denied {
                retry_after: Duration::from_secs_f64(missing / self.refill_per_sec),
            })
        }
    }

    /// Drops buckets that have been idle long enough to be full again; returns how many.
    pub fn evict_idle(&mut self, now: Instant) -> usize {
        let before = self.buckets.len();
        let (burst, rate) = (self.burst, self.refill_per_sec);
        self.buckets.retain(|_, b| {
            let elapsed = now.saturating_duration_since(b.last).as_secs_f64();
            b.tokens + elapsed * rate < burst
        });
        before - self.buckets.len()
    }

    pub fn tracked_keys(&self) -> usize {
        self.buckets.len()
    }
}
