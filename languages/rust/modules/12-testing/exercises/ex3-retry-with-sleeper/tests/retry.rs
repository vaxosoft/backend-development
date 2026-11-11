use std::cell::{Cell, RefCell};
use std::time::Duration;

use m12_ex3_retry_with_sleeper::{Attempt, RetryError, RetryPolicy, Sleeper, TokioSleeper, retry};

/// Records the requested delays and returns immediately.
#[derive(Default)]
struct RecordingSleeper {
    delays: RefCell<Vec<Duration>>,
}

impl Sleeper for RecordingSleeper {
    async fn sleep(&self, duration: Duration) {
        self.delays.borrow_mut().push(duration);
    }
}

fn ms(n: u64) -> Duration {
    Duration::from_millis(n)
}

fn policy(max_attempts: u32) -> RetryPolicy {
    RetryPolicy {
        max_attempts,
        base_delay: ms(10),
        max_delay: Duration::from_secs(1),
    }
}

#[test]
fn delays_double_and_are_capped() {
    let p = RetryPolicy {
        max_attempts: 10,
        base_delay: ms(10),
        max_delay: ms(50),
    };
    let delays: Vec<_> = (1..=6).map(|n| p.delay_for(n)).collect();
    assert_eq!(delays, [ms(10), ms(20), ms(40), ms(50), ms(50), ms(50)]);
}

#[test]
fn delay_calculation_cannot_overflow() {
    let p = RetryPolicy {
        max_attempts: u32::MAX,
        base_delay: Duration::from_secs(1),
        max_delay: Duration::from_secs(30),
    };
    for retry_number in [0, 1, 31, 32, 33, 64, 1_000, u32::MAX] {
        assert!(p.delay_for(retry_number) <= Duration::from_secs(30));
    }
    assert_eq!(p.delay_for(u32::MAX), Duration::from_secs(30));
}

#[tokio::test]
async fn success_on_the_first_attempt_never_sleeps() {
    let sleeper = RecordingSleeper::default();
    let result: Result<&str, RetryError<&str>> =
        retry(&policy(3), &sleeper, |_| async { Ok("fine") }).await;
    assert_eq!(result, Ok("fine"));
    assert!(sleeper.delays.borrow().is_empty());
}

#[tokio::test]
async fn retries_with_backoff_until_it_succeeds() {
    let sleeper = RecordingSleeper::default();
    let seen = RefCell::new(Vec::new());
    let result = retry(&policy(5), &sleeper, |attempt| {
        seen.borrow_mut().push(attempt);
        async move {
            if attempt < 3 {
                Err(Attempt::Retryable("503"))
            } else {
                Ok(attempt * 100)
            }
        }
    })
    .await;

    assert_eq!(result, Ok(300));
    assert_eq!(*seen.borrow(), [1, 2, 3]);
    assert_eq!(*sleeper.delays.borrow(), [ms(10), ms(20)]);
}

#[tokio::test]
async fn exhaustion_reports_the_last_error_and_does_not_sleep_after_it() {
    let sleeper = RecordingSleeper::default();
    let calls = Cell::new(0);
    let result: Result<(), _> = retry(&policy(4), &sleeper, |attempt| {
        calls.set(calls.get() + 1);
        async move { Err(Attempt::Retryable(format!("failure {attempt}"))) }
    })
    .await;

    assert_eq!(
        result,
        Err(RetryError::Exhausted {
            attempts: 4,
            last: "failure 4".to_string()
        })
    );
    assert_eq!(calls.get(), 4);
    assert_eq!(*sleeper.delays.borrow(), [ms(10), ms(20), ms(40)]);
}

#[tokio::test]
async fn fatal_errors_stop_immediately() {
    let sleeper = RecordingSleeper::default();
    let result: Result<(), _> = retry(&policy(5), &sleeper, |attempt| async move {
        if attempt == 1 {
            Err(Attempt::Retryable("timeout"))
        } else {
            Err(Attempt::Fatal("404"))
        }
    })
    .await;

    assert_eq!(
        result,
        Err(RetryError::Fatal {
            attempts: 2,
            error: "404"
        })
    );
    assert_eq!(*sleeper.delays.borrow(), [ms(10)]);
}

#[tokio::test]
async fn zero_max_attempts_still_tries_once() {
    let sleeper = RecordingSleeper::default();
    let calls = Cell::new(0);
    let result: Result<(), _> = retry(&policy(0), &sleeper, |_| {
        calls.set(calls.get() + 1);
        async { Err(Attempt::Retryable("nope")) }
    })
    .await;
    assert_eq!(
        result,
        Err(RetryError::Exhausted {
            attempts: 1,
            last: "nope"
        })
    );
    assert_eq!(calls.get(), 1);
}

#[tokio::test]
async fn the_real_sleeper_waits_at_least_the_requested_time() {
    let started = std::time::Instant::now();
    TokioSleeper.sleep(ms(5)).await;
    assert!(started.elapsed() >= ms(5));
}
