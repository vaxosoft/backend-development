//! A token-bucket rate limiter with an injected clock, so it is testable and deterministic.

use std::time::{Duration, Instant};

struct TokenBucket {
    capacity: f64,
    tokens: f64,
    refill_per_sec: f64,
    last: Instant,
}

impl TokenBucket {
    fn new(capacity: u32, refill_per_sec: f64, now: Instant) -> Self {
        Self {
            capacity: f64::from(capacity),
            tokens: f64::from(capacity),
            refill_per_sec,
            last: now,
        }
    }

    /// Takes one token, or reports how long until one is available.
    fn try_take(&mut self, now: Instant) -> Result<(), Duration> {
        let elapsed = now.saturating_duration_since(self.last).as_secs_f64();
        self.tokens = (self.tokens + elapsed * self.refill_per_sec).min(self.capacity);
        self.last = now;
        if self.tokens >= 1.0 {
            self.tokens -= 1.0;
            Ok(())
        } else {
            Err(Duration::from_secs_f64(
                (1.0 - self.tokens) / self.refill_per_sec,
            ))
        }
    }
}

fn main() {
    let start = Instant::now();
    let at = |ms: u64| start + Duration::from_millis(ms);

    // burst of 3, one new token every 2 seconds
    let mut bucket = TokenBucket::new(3, 0.5, start);

    for ms in [
        0, 0, 0, 0, 1_000, 2_000, 2_100, 10_000, 10_000, 10_000, 10_000,
    ] {
        match bucket.try_take(at(ms)) {
            Ok(()) => println!("t={ms:>5}ms: allowed"),
            Err(wait) => println!(
                "t={ms:>5}ms: denied, Retry-After: {}",
                wait.as_secs_f64().ceil()
            ),
        }
    }
}
