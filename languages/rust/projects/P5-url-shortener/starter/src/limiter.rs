//! A token bucket per user.

use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Duration;

/// Each user owns a bucket holding up to `burst` tokens that refills at
/// `refill_per_sec`. A request takes one token.
pub struct RateLimiter {
    // TODO: add the per-user bucket state (tokens + last refill time).
    _buckets: Mutex<HashMap<i64, ()>>,
}

impl RateLimiter {
    pub fn new(_burst: u32, _refill_per_sec: f64) -> Self {
        Self {
            _buckets: Mutex::new(HashMap::new()),
        }
    }

    /// TODO: take a token for `user_id`. `Ok(())` when allowed; otherwise `Err(wait)` with
    /// the time until a token is available. A new user starts with a full bucket.
    /// Never hold the mutex across an `.await` (there is none here, keep it that way).
    pub fn check(&self, _user_id: i64) -> Result<(), Duration> {
        todo!("token bucket")
    }
}
