use std::collections::BTreeMap;
use std::sync::Arc;
use std::thread;

use m06_ex1_shared_metrics::{Metrics, record_in_parallel};

fn assert_send_sync<T: Send + Sync>() {}

#[test]
fn metrics_can_be_shared_between_threads() {
    assert_send_sync::<Metrics>();
}

#[test]
fn counts_single_threaded() {
    let m = Metrics::new();
    for status in [200, 200, 404, 500, 503] {
        m.record(status);
    }
    let s = m.snapshot();
    assert_eq!(s.requests, 5);
    assert_eq!(s.server_errors, 2);
    assert_eq!(
        s.by_status,
        BTreeMap::from([(200, 2), (404, 1), (500, 1), (503, 1)])
    );
}

#[test]
fn no_updates_are_lost_under_contention() {
    let metrics = Arc::new(Metrics::new());
    let handles: Vec<_> = (0..8)
        .map(|_| {
            let metrics = Arc::clone(&metrics);
            thread::spawn(move || {
                for i in 0..1000 {
                    metrics.record(if i % 10 == 0 { 500 } else { 200 });
                }
            })
        })
        .collect();
    for h in handles {
        h.join().unwrap();
    }
    let s = metrics.snapshot();
    assert_eq!(s.requests, 8000);
    assert_eq!(s.server_errors, 800);
    assert_eq!(s.by_status, BTreeMap::from([(200, 7200), (500, 800)]));
}

#[test]
fn record_in_parallel_waits_for_every_batch() {
    let metrics = Arc::new(Metrics::new());
    let batches = vec![vec![200; 500], vec![404; 300], vec![503; 200], vec![]];
    record_in_parallel(&metrics, batches);
    let s = metrics.snapshot();
    assert_eq!(s.requests, 1000);
    assert_eq!(s.server_errors, 200);
    assert_eq!(s.by_status.get(&404), Some(&300));
    assert_eq!(
        Arc::strong_count(&metrics),
        1,
        "threads must have released their Arc"
    );
}
