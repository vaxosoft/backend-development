//! Exercise 1: shared state between OS threads.
//!
//! `Metrics` is updated from many threads through a shared `&Metrics`
//! (`record` takes `&self`), so it needs interior mutability that is `Sync`:
//! atomics for the plain counters, a `Mutex` for the per-status map.
//! Implement `record`, `snapshot` and `record_in_parallel`.

use std::collections::BTreeMap;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;

#[derive(Debug, Default)]
pub struct Metrics {
    requests: AtomicU64,
    server_errors: AtomicU64,
    by_status: Mutex<BTreeMap<u16, u64>>,
}

#[derive(Debug, PartialEq, Eq)]
pub struct Snapshot {
    pub requests: u64,
    /// Responses with status 500..=599.
    pub server_errors: u64,
    pub by_status: BTreeMap<u16, u64>,
}

impl Metrics {
    pub fn new() -> Metrics {
        Metrics::default()
    }

    /// Counts one response with this HTTP status.
    pub fn record(&self, status: u16) {
        self.requests.fetch_add(1, Ordering::Relaxed);
        if (500..600).contains(&status) {
            self.server_errors.fetch_add(1, Ordering::Relaxed);
        }
        // Poisoning only happens if another thread panicked while holding the lock;
        // the map is still consistent, so keep counting.
        let mut map = self.by_status.lock().unwrap_or_else(|e| e.into_inner());
        *map.entry(status).or_insert(0) += 1;
    }

    pub fn snapshot(&self) -> Snapshot {
        Snapshot {
            requests: self.requests.load(Ordering::Relaxed),
            server_errors: self.server_errors.load(Ordering::Relaxed),
            by_status: self
                .by_status
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .clone(),
        }
    }
}

/// Spawns one OS thread per batch, records every status of that batch from
/// inside the thread, and waits for all threads to finish.
pub fn record_in_parallel(metrics: &Arc<Metrics>, batches: Vec<Vec<u16>>) {
    let handles: Vec<_> = batches
        .into_iter()
        .map(|batch| {
            let metrics = Arc::clone(metrics);
            thread::spawn(move || {
                for status in batch {
                    metrics.record(status);
                }
            })
        })
        .collect();
    for handle in handles {
        handle.join().expect("worker thread panicked");
    }
}
