//! Timeouts, `select!`, cooperative cancellation with `CancellationToken`, and aborting tasks.

use std::time::Duration;

use tokio::sync::mpsc;
use tokio::time::{sleep, timeout};
use tokio_util::sync::CancellationToken;

/// Prints when it is dropped: shows that cancelling a future drops everything it owns.
struct Cleanup(&'static str);

impl Drop for Cleanup {
    fn drop(&mut self) {
        println!("cleanup: {}", self.0);
    }
}

#[tokio::main]
async fn main() {
    // Timeout: the inner future is dropped (cancelled) when the deadline passes.
    let slow = timeout(Duration::from_millis(20), sleep(Duration::from_secs(30))).await;
    println!("timeout: {:?}", slow.map_err(|e| e.to_string()));
    let quick = timeout(Duration::from_secs(5), async { 42 }).await;
    println!("timeout: {quick:?}");

    // select!: the first branch to finish wins, the others are dropped.
    tokio::select! {
        _ = sleep(Duration::from_millis(100)) => println!("select: slow branch"),
        _ = sleep(Duration::from_millis(5)) => println!("select: fast branch"),
    }

    // Aborting a task cancels it at its next `.await`; its locals are dropped.
    let task = tokio::spawn(async {
        let _guard = Cleanup("aborted task");
        sleep(Duration::from_secs(30)).await;
    });
    sleep(Duration::from_millis(10)).await; // let it start
    task.abort();
    let err = task.await.unwrap_err();
    println!("abort: is_cancelled={}", err.is_cancelled());

    // CancellationToken: cooperative shutdown. The worker decides where to stop.
    let token = CancellationToken::new();
    let (tx, mut rx) = mpsc::channel::<u32>(8);
    let worker = tokio::spawn({
        let token = token.clone();
        async move {
            let mut handled = 0;
            loop {
                tokio::select! {
                    _ = token.cancelled() => break,
                    item = rx.recv() => match item {
                        Some(n) => { println!("worker handled {n}"); handled += 1; }
                        None => break,
                    },
                }
            }
            handled
        }
    });
    for n in 1..=3 {
        tx.send(n).await.unwrap();
    }
    sleep(Duration::from_millis(30)).await;
    token.cancel();
    println!("worker stopped after {} items", worker.await.unwrap());

    // Child tokens: cancelling the parent cancels every child, not the other way round.
    let parent = CancellationToken::new();
    let child = parent.child_token();
    child.cancel();
    println!(
        "child cancelled, parent cancelled? {}",
        parent.is_cancelled()
    );
    parent.cancel();
    println!(
        "parent cancelled, child cancelled? {}",
        child.is_cancelled()
    );
}
