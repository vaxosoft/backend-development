use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use m06_ex3_worker_pool::{SubmitError, WorkerPool};
use tokio::sync::{Barrier, Notify, Semaphore};
use tokio::time::{sleep, timeout};

#[tokio::test]
async fn every_accepted_job_runs_before_shutdown_returns() {
    let seen = Arc::new(Mutex::new(Vec::new()));
    let sink = Arc::clone(&seen);
    let pool = WorkerPool::new(3, 100, move |n: u32| {
        let sink = Arc::clone(&sink);
        async move {
            sleep(Duration::from_millis(1)).await;
            sink.lock().unwrap().push(n);
        }
    });
    for n in 0..50 {
        pool.submit(n).unwrap();
    }
    pool.shutdown().await;
    let mut seen = seen.lock().unwrap().clone();
    seen.sort_unstable();
    assert_eq!(seen, (0..50).collect::<Vec<_>>());
}

#[tokio::test]
async fn a_full_queue_rejects_instead_of_blocking() {
    let started = Arc::new(Notify::new());
    let gate = Arc::new(Semaphore::new(0));
    let done = Arc::new(AtomicUsize::new(0));
    let (s, g, d) = (Arc::clone(&started), Arc::clone(&gate), Arc::clone(&done));
    let pool = WorkerPool::new(1, 1, move |_: u32| {
        let (s, g, d) = (Arc::clone(&s), Arc::clone(&g), Arc::clone(&d));
        async move {
            s.notify_one();
            g.acquire().await.unwrap().forget();
            d.fetch_add(1, Ordering::SeqCst);
        }
    });

    pool.submit(1).unwrap();
    timeout(Duration::from_secs(2), started.notified())
        .await
        .unwrap();
    pool.submit(2).unwrap();
    assert_eq!(pool.submit(3), Err(SubmitError::Full));

    gate.add_permits(10);
    pool.shutdown().await;
    assert_eq!(
        done.load(Ordering::SeqCst),
        2,
        "queued job must still run, rejected one must not"
    );
}

#[tokio::test]
async fn workers_really_run_in_parallel() {
    // Three jobs wait for each other; a sequential pool would never get past the barrier.
    let barrier = Arc::new(Barrier::new(3));
    let b = Arc::clone(&barrier);
    let pool = WorkerPool::new(3, 3, move |_: u8| {
        let b = Arc::clone(&b);
        async move {
            b.wait().await;
        }
    });
    for i in 0..3 {
        pool.submit(i).unwrap();
    }
    timeout(Duration::from_secs(2), pool.shutdown())
        .await
        .expect("3 workers should have released the barrier");
}

#[tokio::test]
async fn shutdown_drains_the_queue() {
    let done = Arc::new(AtomicUsize::new(0));
    let d = Arc::clone(&done);
    let pool = WorkerPool::new(1, 10, move |_: u8| {
        let d = Arc::clone(&d);
        async move {
            sleep(Duration::from_millis(2)).await;
            d.fetch_add(1, Ordering::SeqCst);
        }
    });
    for i in 0..5 {
        pool.submit(i).unwrap();
    }
    pool.shutdown().await;
    assert_eq!(done.load(Ordering::SeqCst), 5);
}
