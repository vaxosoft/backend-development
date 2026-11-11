//! A token bucket per user.

use std::collections::HashMap;
use std::sync::{Mutex, PoisonError};
use std::time::{Duration, Instant};

struct Bucket {
    tokens: f64,
    last_refill: Instant,
}

/// Each user owns a bucket holding up to `burst` tokens that refills at
/// `refill_per_sec`. A request takes one token.
pub struct RateLimiter {
    burst: f64,
    refill_per_sec: f64,
    buckets: Mutex<HashMap<i64, Bucket>>,
}

impl RateLimiter {
    pub fn new(burst: u32, refill_per_sec: f64) -> Self {
        Self {
            burst: f64::from(burst),
            refill_per_sec,
            buckets: Mutex::new(HashMap::new()),
        }
    }

    /// `Ok(())` when a token was taken, otherwise `Err(wait)` until the next one.
    pub fn check(&self, user_id: i64) -> Result<(), Duration> {
        let now = Instant::now();
        let mut buckets = self.buckets.lock().unwrap_or_else(PoisonError::into_inner);
        let bucket = buckets.entry(user_id).or_insert(Bucket {
            tokens: self.burst,
            last_refill: now,
        });
        let elapsed = now.duration_since(bucket.last_refill).as_secs_f64();
        bucket.tokens = (bucket.tokens + elapsed * self.refill_per_sec).min(self.burst);
        bucket.last_refill = now;
        if bucket.tokens >= 1.0 {
            bucket.tokens -= 1.0;
            return Ok(());
        }
        // With no refill the wait is infinite; cap it so `Duration` stays valid.
        let wait = (1.0 - bucket.tokens) / self.refill_per_sec;
        Err(Duration::from_secs_f64(wait.min(3600.0)))
    }
}
