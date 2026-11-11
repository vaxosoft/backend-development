use std::time::{Duration, Instant};

use m11_ex2_token_bucket::RateLimiter;

fn ms(base: Instant, n: u64) -> Instant {
    base + Duration::from_millis(n)
}

#[test]
fn allows_a_burst_then_denies() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(3, 1.0);
    for _ in 0..3 {
        assert!(rl.check("alice", t0).is_ok());
    }
    assert!(rl.check("alice", t0).is_err());
}

#[test]
fn keys_do_not_share_tokens() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(1, 1.0);
    assert!(rl.check("alice", t0).is_ok());
    assert!(rl.check("alice", t0).is_err());
    assert!(rl.check("bob", t0).is_ok());
}

#[test]
fn tokens_refill_with_elapsed_time() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(2, 2.0); // one token per 500 ms
    assert!(rl.check("a", t0).is_ok());
    assert!(rl.check("a", t0).is_ok());
    assert!(rl.check("a", ms(t0, 400)).is_err());
    assert!(rl.check("a", ms(t0, 500)).is_ok());
    assert!(rl.check("a", ms(t0, 500)).is_err());
}

#[test]
fn refill_is_capped_at_the_burst() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(2, 1.0);
    assert!(rl.check("a", t0).is_ok());
    // an hour of idling must not bank more than `burst` tokens
    let later = t0 + Duration::from_secs(3600);
    assert!(rl.check("a", later).is_ok());
    assert!(rl.check("a", later).is_ok());
    assert!(rl.check("a", later).is_err());
}

#[test]
fn denial_reports_how_long_to_wait() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(1, 0.5); // one token per 2 s
    assert!(rl.check("a", t0).is_ok());

    let denied = rl.check("a", t0).unwrap_err();
    assert_eq!(denied.retry_after, Duration::from_secs(2));
    assert_eq!(denied.retry_after_secs(), 2);

    let denied = rl.check("a", ms(t0, 500)).unwrap_err();
    assert_eq!(denied.retry_after, Duration::from_millis(1500));
    assert_eq!(denied.retry_after_secs(), 2); // rounded up
}

#[test]
fn retry_after_is_at_least_one_second() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(1, 100.0);
    assert!(rl.check("a", t0).is_ok());
    let denied = rl.check("a", t0).unwrap_err();
    assert_eq!(denied.retry_after_secs(), 1);
}

#[test]
fn a_clock_going_backwards_does_not_mint_tokens() {
    let t0 = Instant::now() + Duration::from_secs(10);
    let mut rl = RateLimiter::new(1, 1.0);
    assert!(rl.check("a", t0).is_ok());
    assert!(rl.check("a", t0 - Duration::from_secs(5)).is_err());
}

#[test]
fn idle_buckets_are_evicted_but_busy_ones_stay() {
    let t0 = Instant::now();
    let mut rl = RateLimiter::new(2, 1.0); // takes 2 s to refill one token
    assert!(rl.check("idle", t0).is_ok());
    assert!(rl.check("busy", t0).is_ok());
    assert!(rl.check("busy", ms(t0, 2500)).is_ok());
    assert!(rl.check("busy", ms(t0, 2500)).is_ok());
    assert_eq!(rl.tracked_keys(), 2);

    // "idle" has refilled completely; "busy" is nearly empty
    assert_eq!(rl.evict_idle(ms(t0, 3000)), 1);
    assert_eq!(rl.tracked_keys(), 1);
    assert!(rl.check("busy", ms(t0, 3000)).is_err());
}
