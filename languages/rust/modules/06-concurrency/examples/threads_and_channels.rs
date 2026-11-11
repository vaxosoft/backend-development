//! OS threads: spawn/join, scoped threads, `Arc<Mutex<_>>`, atomics and `mpsc` channels.

use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, mpsc};
use std::thread;

fn main() {
    // `spawn` takes ownership of what the closure uses (`move`); `join` returns the result.
    let handle = thread::spawn(|| (1..=10).sum::<u32>());
    println!("sum from a thread: {}", handle.join().unwrap());

    // Scoped threads may borrow local data because the scope joins them before returning.
    let data = [1, 2, 3, 4, 5, 6];
    let (left, right) = data.split_at(3);
    let (a, b) = thread::scope(|s| {
        let a = s.spawn(|| left.iter().sum::<i32>());
        let b = s.spawn(|| right.iter().sum::<i32>());
        (a.join().unwrap(), b.join().unwrap())
    });
    println!("borrowed halves: {a} + {b}");

    // Shared mutable state: Arc for shared ownership, Mutex for exclusive access.
    let counter = Arc::new(Mutex::new(0_u32));
    let handles: Vec<_> = (0..4)
        .map(|_| {
            let counter = Arc::clone(&counter);
            thread::spawn(move || {
                for _ in 0..1000 {
                    *counter.lock().unwrap() += 1;
                }
            })
        })
        .collect();
    for h in handles {
        h.join().unwrap();
    }
    println!("mutex counter: {}", *counter.lock().unwrap());

    // A plain counter does not need a lock: an atomic is enough.
    let hits = Arc::new(AtomicU64::new(0));
    thread::scope(|s| {
        for _ in 0..4 {
            s.spawn(|| {
                for _ in 0..1000 {
                    hits.fetch_add(1, Ordering::Relaxed);
                }
            });
        }
    });
    println!("atomic counter: {}", hits.load(Ordering::Relaxed));

    // Message passing: producers own their sender clone; the loop ends when all senders are gone.
    let (tx, rx) = mpsc::channel();
    let producers: Vec<_> = (0..3)
        .map(|id| {
            let tx = tx.clone();
            thread::spawn(move || {
                for n in 0..2 {
                    tx.send(id * 10 + n).unwrap();
                }
            })
        })
        .collect();
    drop(tx); // without this the receiver below would wait forever
    let mut received: Vec<i32> = rx.iter().collect();
    received.sort_unstable();
    println!("received: {received:?}");
    for p in producers {
        p.join().unwrap();
    }

    // A panic stays inside its thread; `join` reports it as an `Err`.
    std::panic::set_hook(Box::new(|_| {})); // keep the demo output free of the panic message
    let result = thread::spawn(|| -> u32 { panic!("worker failed") }).join();
    println!("panicked thread: is_err={}", result.is_err());
}
