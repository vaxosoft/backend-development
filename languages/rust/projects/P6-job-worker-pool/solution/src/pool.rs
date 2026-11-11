//! The bounded queue, the worker tasks and the job registry.

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
    pub fn submit(&self, spec: JobSpec) -> Result<String, SubmitError> {
        let queue = lock(&self.shared.queue);
        let tx = queue.as_ref().ok_or(SubmitError::ShuttingDown)?;
        let id = Uuid::now_v7().to_string();
        let entry = Entry {
            view: JobView {
                id: id.clone(),
                kind: spec.kind(),
                status: JobStatus::Queued,
                attempts: 0,
                result: None,
                error: None,
                created_at: Utc::now(),
            },
            spec,
            seq: self.shared.next_seq.fetch_add(1, Ordering::Relaxed),
            cancel: CancellationToken::new(),
        };
        lock(&self.shared.jobs).insert(id.clone(), entry);
        match tx.try_send(id.clone()) {
            Ok(()) => {
                info!(%id, "job queued");
                Ok(id)
            }
            Err(e) => {
                lock(&self.shared.jobs).remove(&id);
                Err(match e {
                    mpsc::error::TrySendError::Full(_) => SubmitError::QueueFull,
                    mpsc::error::TrySendError::Closed(_) => SubmitError::ShuttingDown,
                })
            }
        }
    }

    /// Cancels a queued or running job.
    ///
    /// The status becomes `cancelled` immediately and the job's token is
    /// cancelled so that a running attempt is dropped. Unknown id -> `NotFound`,
    /// already succeeded/failed/cancelled -> `Finished`.
    pub fn cancel(&self, id: &str) -> Result<(), CancelError> {
        let mut jobs = lock(&self.shared.jobs);
        let entry = jobs.get_mut(id).ok_or(CancelError::NotFound)?;
        if entry.view.status.is_finished() {
            return Err(CancelError::Finished);
        }
        entry.view.status = JobStatus::Cancelled;
        entry.cancel.cancel();
        Ok(())
    }

    /// Graceful shutdown of the pool.
    ///
    /// 1. close the queue (new submissions fail, queued jobs are still delivered)
    /// 2. wait for the workers for at most `cfg.drain_timeout`
    /// 3. on timeout cancel every unfinished job, then wait for the workers again
    ///
    /// Calling it twice must be harmless.
    pub async fn drain(&self) {
        lock(&self.shared.queue).take();
        let mut workers = std::mem::take(&mut *lock(&self.shared.workers));
        if timeout(self.shared.cfg.drain_timeout, join_all(&mut workers))
            .await
            .is_err()
        {
            warn!("drain timeout reached, cancelling unfinished jobs");
            let ids: Vec<String> = lock(&self.shared.jobs).keys().cloned().collect();
            for id in ids {
                let _ = self.cancel(&id);
            }
            join_all(&mut workers).await;
        }
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
async fn run_job(shared: &Shared, id: &str) {
    let Some((spec, token)) = shared.begin(id) else {
        return;
    };
    let cfg = &shared.cfg;
    let max_attempts = cfg.max_attempts.max(1);
    for attempt in 1..=max_attempts {
        shared.set_attempts(id, attempt);
        let outcome = tokio::select! {
            _ = token.cancelled() => return,
            res = timeout(cfg.job_timeout, execute(&spec)) => match res {
                Ok(outcome) => outcome,
                Err(_) => Err(format!("timed out after {}ms", cfg.job_timeout.as_millis())),
            },
        };
        match outcome {
            Ok(result) => {
                info!(attempt, "job succeeded");
                return shared.finish(id, JobStatus::Succeeded, Some(result), None);
            }
            Err(error) if attempt == max_attempts => {
                warn!(attempt, %error, "job failed for good");
                return shared.finish(id, JobStatus::Failed, None, Some(error));
            }
            Err(error) => {
                warn!(attempt, %error, "attempt failed, will retry");
                tokio::select! {
                    _ = token.cancelled() => return,
                    _ = sleep(cfg.retry_delay) => {}
                }
            }
        }
    }
}
