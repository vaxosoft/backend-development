//! Exercise 1: shared state between OS threads.
//!
//! `Metrics` is updated from many threads through a shared `&Metrics`
//! (`record` takes `&self`), so it needs interior mutability that is `Sync`:
//! atomics for the plain counters, a `Mutex` for the per-status map.
//! Implement `record`, `snapshot` and `record_in_parallel`.

use std::collections::BTreeMap;
use std::sync::Arc;
use std::sync::Mutex;
use std::sync::atomic::AtomicU64;

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
        let _ = status;
        todo!("bump the atomics and the map entry")
    }

    pub fn snapshot(&self) -> Snapshot {
        todo!("read the atomics and clone the map")
    }
}

/// Spawns one OS thread per batch, records every status of that batch from
/// inside the thread, and waits for all threads to finish.
pub fn record_in_parallel(metrics: &Arc<Metrics>, batches: Vec<Vec<u16>>) {
    let _ = (metrics, batches);
    todo!("thread::spawn with a cloned Arc per batch, then join them all")
}
