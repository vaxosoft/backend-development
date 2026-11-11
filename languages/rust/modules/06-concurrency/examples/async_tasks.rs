//! Async tasks on Tokio: `spawn`, `join!`, `JoinSet`, shared state, and `spawn_blocking`.

use std::sync::{Arc, Mutex};
use std::time::Duration;

use tokio::task::JoinSet;
use tokio::time::sleep;

async fn fetch(name: &'static str, ms: u64) -> String {
    sleep(Duration::from_millis(ms)).await;
    format!("{name} after {ms}ms")
}

/// Pretend CPU-bound work: it never awaits, so it would stall a runtime thread.
fn count_primes(limit: u32) -> usize {
    (2..limit)
        .filter(|n| (2..*n).take_while(|d| d * d <= *n).all(|d| n % d != 0))
        .count()
}

#[tokio::main]
async fn main() {
    // `spawn` starts a task right away; awaiting the handle gives its output.
    let handle = tokio::spawn(fetch("spawned", 5));
    println!("{}", handle.await.unwrap());

    // `join!` runs futures concurrently in the *same* task and waits for all of them.
    let (a, b) = tokio::join!(fetch("a", 20), fetch("b", 5));
    println!("join!: {a}, {b}");

    // JoinSet: many tasks, results in completion order, all aborted if the set is dropped.
    let mut set = JoinSet::new();
    set.spawn(fetch("slow", 60));
    set.spawn(fetch("fast", 10));
    set.spawn(fetch("medium", 35));
    while let Some(done) = set.join_next().await {
        println!("completed: {}", done.unwrap());
    }

    // Shared state: a std Mutex is fine when the guard is never held across an `.await`.
    let log = Arc::new(Mutex::new(Vec::new()));
    let mut set = JoinSet::new();
    for id in 0..5 {
        let log = Arc::clone(&log);
        set.spawn(async move {
            sleep(Duration::from_millis(1)).await;
            log.lock().unwrap().push(id); // guard dropped at the end of the statement
        });
    }
    while set.join_next().await.is_some() {}
    let mut log = log.lock().unwrap().clone();
    log.sort_unstable();
    println!("log: {log:?}");

    // CPU-bound or blocking work belongs on the blocking pool.
    let primes = tokio::task::spawn_blocking(|| count_primes(50_000))
        .await
        .unwrap();
    println!("primes below 50000: {primes}");

    // A panicking task does not take the runtime down; the JoinError says what happened.
    std::panic::set_hook(Box::new(|_| {})); // keep the demo output free of the panic message
    let err = tokio::spawn(async { panic!("task failed") })
        .await
        .unwrap_err();
    println!("panicked task: is_panic={}", err.is_panic());
}
