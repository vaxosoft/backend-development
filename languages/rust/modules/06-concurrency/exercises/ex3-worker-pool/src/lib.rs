//! Exercise 3: a fixed worker pool with a bounded queue and graceful shutdown.
//!
//! * `submit` never waits: a full queue is reported as `SubmitError::Full`
//!   (an HTTP layer would answer 429).
//! * `shutdown` stops accepting work, lets the workers finish **every job that
//!   was already accepted**, and returns when they are all done.
//!
//! Hint: one `tokio::sync::mpsc` channel; tokio's `Receiver` has a single
//! consumer, so the workers share it behind an `Arc<tokio::sync::Mutex<_>>`
//! (hold the lock only while waiting for the next job, never while running it).

use std::future::Future;

use tokio::sync::mpsc;
use tokio::task::JoinHandle;

#[derive(Debug, PartialEq, Eq)]
pub enum SubmitError {
    /// The queue is at capacity right now.
    Full,
    /// All workers are gone.
    Closed,
}

pub struct WorkerPool<J> {
    sender: mpsc::Sender<J>,
    workers: Vec<JoinHandle<()>>,
}

impl<J: Send + 'static> WorkerPool<J> {
    /// Starts `workers` tasks (at least 1) sharing a queue of `capacity` jobs (at least 1).
    /// Must be called inside a Tokio runtime.
    pub fn new<F, Fut>(workers: usize, capacity: usize, handler: F) -> WorkerPool<J>
    where
        F: Fn(J) -> Fut + Send + Sync + 'static,
        Fut: Future<Output = ()> + Send + 'static,
    {
        let _ = (workers, capacity, handler);
        todo!("create the channel, share the receiver, spawn the worker loops")
    }

    pub fn submit(&self, job: J) -> Result<(), SubmitError> {
        let _ = job;
        todo!("try_send and translate the error")
    }

    /// Closes the queue and waits until every accepted job has been handled.
    pub async fn shutdown(self) {
        todo!("drop the sender, then await every worker handle")
    }
}
