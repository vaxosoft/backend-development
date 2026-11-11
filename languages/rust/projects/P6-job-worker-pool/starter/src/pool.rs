//! The bounded queue, the worker tasks and the job registry.

// TODO: remove once the stubs below are implemented.
#![allow(dead_code, unused_imports)]

use std::cmp::Reverse;
use std::collections::HashMap;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, MutexGuard, PoisonError};

use chrono::Utc;
use thiserror::Error;
use tokio::sync::mpsc;
use tokio::task::JoinSet;
use tokio::time::{sleep, timeout};
use tokio_util::sync::CancellationToken;
use tracing::{Instrument, info, info_span, warn};
use uuid::Uuid;

use crate::Config;
use crate::exec::execute;
use crate::jobs::{JobSpec, JobStatus, JobView};

#[derive(Debug, Error, PartialEq, Eq)]
pub enum SubmitError {
    #[error("the queue is full")]
    QueueFull,
    #[error("the pool is shutting down")]
    ShuttingDown,
}

#[derive(Debug, Error, PartialEq, Eq)]
pub enum CancelError {
    #[error("no such job")]
    NotFound,
    #[error("the job already finished")]
    Finished,
}

struct Entry {
    view: JobView,
    spec: JobSpec,
    seq: u64,
    cancel: CancellationToken,
}

struct Shared {
    cfg: Config,
    jobs: Mutex<HashMap<String, Entry>>,
    next_seq: AtomicU64,
    // `None` once the pool is draining; dropping the sender closes the queue.
    queue: Mutex<Option<mpsc::Sender<String>>>,
    workers: Mutex<JoinSet<()>>,
}

type SharedReceiver = Arc<tokio::sync::Mutex<mpsc::Receiver<String>>>;

/// Handle to the pool; cheap to clone.
#[derive(Clone)]
pub struct Pool {
    shared: Arc<Shared>,
}

// A poisoned lock only means another thread panicked; the data is still usable.
fn lock<T>(mutex: &Mutex<T>) -> MutexGuard<'_, T> {
    mutex.lock().unwrap_or_else(PoisonError::into_inner)
}

impl Pool {
    /// Creates the bounded queue and spawns `cfg.workers` worker tasks.
    pub fn start(cfg: Config) -> Pool {
        let (tx, rx) = mpsc::channel(cfg.queue_capacity.max(1));
        let rx: SharedReceiver = Arc::new(tokio::sync::Mutex::new(rx));
        let workers = cfg.workers.max(1);
        let shared = Arc::new(Shared {
            cfg,
            jobs: Mutex::new(HashMap::new()),
            next_seq: AtomicU64::new(0),
            queue: Mutex::new(Some(tx)),
            workers: Mutex::new(JoinSet::new()),
        });
        for n in 0..workers {
            lock(&shared.workers).spawn(worker_loop(n, shared.clone(), rx.clone()));
        }
        Pool { shared }
    }

    pub fn get(&self, id: &str) -> Option<JobView> {
        lock(&self.shared.jobs).get(id).map(|e| e.view.clone())
    }

    /// All jobs, newest first.
    pub fn list(&self) -> Vec<JobView> {
        let jobs = lock(&self.shared.jobs);
        let mut entries: Vec<&Entry> = jobs.values().collect();
        entries.sort_by_key(|e| Reverse(e.seq));
        entries.into_iter().map(|e| e.view.clone()).collect()
    }

    /// Registers a `queued` job and puts its id on the queue without waiting.
    ///
    /// * the registry entry must exist before the id is sent (a worker may pick it up at once)
    /// * `try_send` full -> `QueueFull` and the entry must not stay in the registry
    /// * queue already closed by `drain` -> `ShuttingDown`
    pub fn submit(&self, _spec: JobSpec) -> Result<String, SubmitError> {
        todo!("TODO: register the job and try_send its id")
    }

    /// Cancels a queued or running job.
    ///
    /// The status becomes `cancelled` immediately and the job's token is
    /// cancelled so that a running attempt is dropped. Unknown id -> `NotFound`,
    /// already succeeded/failed/cancelled -> `Finished`.
    pub fn cancel(&self, _id: &str) -> Result<(), CancelError> {
        todo!("TODO: cancel a queued or running job")
    }

    /// Graceful shutdown of the pool.
    ///
    /// 1. close the queue (new submissions fail, queued jobs are still delivered)
    /// 2. wait for the workers for at most `cfg.drain_timeout`
    /// 3. on timeout cancel every unfinished job, then wait for the workers again
    ///
    /// Calling it twice must be harmless.
    pub async fn drain(&self) {
        todo!("TODO: close the queue, wait for the workers, cancel stragglers")
    }
}

async fn join_all(workers: &mut JoinSet<()>) {
    while workers.join_next().await.is_some() {}
}

impl Shared {
    /// `queued` -> `running`. `None` if the job was cancelled while it waited.
    fn begin(&self, id: &str) -> Option<(JobSpec, CancellationToken)> {
        let mut jobs = lock(&self.jobs);
        let entry = jobs.get_mut(id)?;
        if entry.view.status != JobStatus::Queued {
            return None;
        }
        entry.view.status = JobStatus::Running;
        Some((entry.spec.clone(), entry.cancel.clone()))
    }

    fn set_attempts(&self, id: &str, attempts: u32) {
        if let Some(entry) = lock(&self.jobs).get_mut(id) {
            entry.view.attempts = attempts;
        }
    }

    /// Records the final state unless the job was cancelled in the meantime.
    fn finish(&self, id: &str, status: JobStatus, result: Option<String>, error: Option<String>) {
        if let Some(entry) = lock(&self.jobs).get_mut(id)
            && !entry.view.status.is_finished()
        {
            entry.view.status = status;
            entry.view.result = result;
            entry.view.error = error;
        }
    }
}

async fn worker_loop(n: usize, shared: Arc<Shared>, rx: SharedReceiver) {
    loop {
        // The async mutex lets the workers take turns waiting on the one receiver.
        let next = rx.lock().await.recv().await;
        let Some(id) = next else { break };
        run_job(&shared, &id)
            .instrument(info_span!("job", %id, worker = n))
            .await;
    }
}

/// Runs a dequeued job to its final state.
///
/// * skip it if `begin` says it was cancelled while queued
/// * attempt up to `cfg.max_attempts` times (`set_attempts` before each one)
/// * one attempt = `execute(&spec)` bounded by `cfg.job_timeout`; a timeout is a failure
/// * a failed attempt that is not the last waits `cfg.retry_delay`
/// * cancellation (`token.cancelled()`) must interrupt both the attempt and the
///   delay; `cancel` has already recorded the status, so just return
/// * the final state goes through `finish`
async fn run_job(_shared: &Shared, _id: &str) {
    todo!("TODO: attempts, timeout, retry delay, cancellation")
}
