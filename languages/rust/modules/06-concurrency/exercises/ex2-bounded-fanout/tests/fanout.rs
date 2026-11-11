use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use m06_ex2_bounded_fanout::{map_concurrent, map_concurrent_fail_fast};
use tokio::time::{sleep, timeout};

/// Counts how many calls are in flight and remembers the maximum.
#[derive(Default)]
struct Gauge {
    current: AtomicUsize,
    max: AtomicUsize,
}

impl Gauge {
    fn enter(&self) {
        let now = self.current.fetch_add(1, Ordering::SeqCst) + 1;
        self.max.fetch_max(now, Ordering::SeqCst);
    }
    fn leave(&self) {
        self.current.fetch_sub(1, Ordering::SeqCst);
    }
}

#[tokio::test]
async fn results_keep_input_order_even_when_calls_finish_out_of_order() {
    let items: Vec<u64> = (0..6).collect();
    let out = map_concurrent(items, 6, |i| async move {
        sleep(Duration::from_millis((6 - i) * 3)).await;
        i * 10
    })
    .await;
    assert_eq!(out, [0, 10, 20, 30, 40, 50]);
}

#[tokio::test]
async fn never_exceeds_the_limit_but_does_use_it() {
    let gauge = Arc::new(Gauge::default());
    let g = Arc::clone(&gauge);
    let out = map_concurrent((0..12).collect::<Vec<u32>>(), 3, move |i| {
        let g = Arc::clone(&g);
        async move {
            g.enter();
            sleep(Duration::from_millis(5)).await;
            g.leave();
            i
        }
    })
    .await;
    assert_eq!(out.len(), 12);
    assert_eq!(gauge.max.load(Ordering::SeqCst), 3);
}

#[tokio::test]
async fn limit_zero_still_makes_progress() {
    let out = map_concurrent(vec![1, 2, 3], 0, |i| async move { i + 1 }).await;
    assert_eq!(out, [2, 3, 4]);
}

#[tokio::test]
async fn fail_fast_collects_values_in_order_on_success() {
    let out: Result<Vec<u64>, String> =
        map_concurrent_fail_fast((0..5).collect(), 5, |i| async move {
            sleep(Duration::from_millis((5 - i) * 2)).await;
            Ok(i * i)
        })
        .await;
    assert_eq!(out.unwrap(), [0, 1, 4, 9, 16]);
}

/// Dropping a future drops the guards it owns; that is how we observe cancellation.
struct DropCounter(Arc<AtomicUsize>);
impl Drop for DropCounter {
    fn drop(&mut self) {
        self.0.fetch_add(1, Ordering::SeqCst);
    }
}

#[tokio::test]
async fn fail_fast_returns_at_once_and_cancels_the_rest() {
    let started = Arc::new(AtomicUsize::new(0));
    let dropped = Arc::new(AtomicUsize::new(0));
    let (s, d) = (Arc::clone(&started), Arc::clone(&dropped));

    let outcome = timeout(Duration::from_secs(2), async {
        map_concurrent_fail_fast((0..10).collect::<Vec<u32>>(), 3, move |i| {
            let (s, d) = (Arc::clone(&s), Arc::clone(&d));
            async move {
                let _guard = DropCounter(d);
                s.fetch_add(1, Ordering::SeqCst);
                if i == 0 {
                    // Gives the other two calls a chance to start before this one fails.
                    tokio::task::yield_now().await;
                    Err("boom".to_string())
                } else {
                    std::future::pending::<Result<u32, String>>().await
                }
            }
        })
        .await
    })
    .await
    .expect("must not wait for the pending calls");

    assert_eq!(outcome.unwrap_err(), "boom");
    assert_eq!(
        started.load(Ordering::SeqCst),
        3,
        "no new call after the failure"
    );
    assert_eq!(
        dropped.load(Ordering::SeqCst),
        3,
        "in-flight calls were dropped"
    );
}
