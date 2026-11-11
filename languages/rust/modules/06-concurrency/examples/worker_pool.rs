//! Worker pool, fan-out/fan-in, backpressure and bounded concurrency.

use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use tokio::sync::{Mutex, Semaphore, mpsc};
use tokio::task::JoinSet;
use tokio::time::sleep;

#[tokio::main]
async fn main() {
    // Backpressure: a bounded channel tells a fast producer to slow down (or to reject work).
    let (tx, mut rx) = mpsc::channel::<u32>(2);
    tx.try_send(1).unwrap();
    tx.try_send(2).unwrap();
    println!(
        "third try_send: {:?}",
        tx.try_send(3).map_err(|e| e.to_string())
    );
    rx.recv().await;
    println!(
        "after one recv: {:?}",
        tx.try_send(3).map_err(|e| e.to_string())
    );
    drop((tx, rx));

    // Worker pool: N workers share one receiver; results fan in through a second channel.
    let (jobs_tx, jobs_rx) = mpsc::channel::<u32>(4);
    let (results_tx, mut results_rx) = mpsc::channel::<(usize, u32)>(4);
    let jobs_rx = Arc::new(Mutex::new(jobs_rx));
    let mut workers = JoinSet::new();
    for worker_id in 0..3 {
        let jobs_rx = Arc::clone(&jobs_rx);
        let results_tx = results_tx.clone();
        workers.spawn(async move {
            loop {
                // The lock is held only while waiting for the next job, not while working on it.
                let job = jobs_rx.lock().await.recv().await;
                let Some(n) = job else { break };
                sleep(Duration::from_millis(2)).await;
                results_tx.send((worker_id, n * n)).await.unwrap();
            }
        });
    }
    drop(results_tx); // the collector below ends when every worker has dropped its sender

    let producer = tokio::spawn(async move {
        for n in 1..=8 {
            jobs_tx.send(n).await.unwrap(); // waits while the queue is full
        }
        // jobs_tx dropped here: workers see `None` once the queue is drained
    });

    let mut squares = Vec::new();
    while let Some((_worker, square)) = results_rx.recv().await {
        squares.push(square);
    }
    squares.sort_unstable();
    println!("squares: {squares:?}");
    producer.await.unwrap();
    while workers.join_next().await.is_some() {}

    // Bounded concurrency without a pool: a Semaphore limits how many tasks run at once.
    let limit = Arc::new(Semaphore::new(2));
    let current = Arc::new(AtomicUsize::new(0));
    let peak = Arc::new(AtomicUsize::new(0));
    let mut set = JoinSet::new();
    for _ in 0..6 {
        let (limit, current, peak) = (Arc::clone(&limit), Arc::clone(&current), Arc::clone(&peak));
        set.spawn(async move {
            let _permit = limit.acquire().await.unwrap();
            let now = current.fetch_add(1, Ordering::SeqCst) + 1;
            peak.fetch_max(now, Ordering::SeqCst);
            sleep(Duration::from_millis(10)).await;
            current.fetch_sub(1, Ordering::SeqCst);
        });
    }
    while set.join_next().await.is_some() {}
    println!(
        "6 tasks, permits=2, peak in flight: {}",
        peak.load(Ordering::SeqCst)
    );
}
