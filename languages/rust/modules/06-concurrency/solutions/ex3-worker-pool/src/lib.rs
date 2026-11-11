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
use std::sync::Arc;

use tokio::sync::{Mutex, mpsc};
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
        let (sender, receiver) = mpsc::channel(capacity.max(1));
        let receiver = Arc::new(Mutex::new(receiver));
        let handler = Arc::new(handler);
        let workers = (0..workers.max(1))
            .map(|_| {
                let receiver = Arc::clone(&receiver);
                let handler = Arc::clone(&handler);
                tokio::spawn(async move {
                    loop {
                        // The guard is a temporary: released before the handler runs.
                        let next = receiver.lock().await.recv().await;
                        match next {
                            Some(job) => handler(job).await,
                            None => break,
                        }
                    }
                })
            })
            .collect();
        WorkerPool { sender, workers }
    }

    pub fn submit(&self, job: J) -> Result<(), SubmitError> {
        self.sender.try_send(job).map_err(|e| match e {
            mpsc::error::TrySendError::Full(_) => SubmitError::Full,
            mpsc::error::TrySendError::Closed(_) => SubmitError::Closed,
        })
    }

    /// Closes the queue and waits until every accepted job has been handled.
    pub async fn shutdown(self) {
        drop(self.sender);
        for worker in self.workers {
            let _ = worker.await;
        }
    }
}
