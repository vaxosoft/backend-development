# 06. Concurrency in Rust

> Estimated time: 2h | Difficulty: Intermediate-Advanced | Prerequisites: 01 (ownership, closures, traits), 02 (async handlers)

## Learning Outcomes
After this module you can:
- [ ] Choose between OS threads and async tasks, and spawn, join and cancel both
- [ ] Share state safely (`Arc<Mutex<_>>`, `RwLock`, atomics) or pass messages (`mpsc`, `oneshot`)
- [ ] Build a bounded worker pool and a bounded fan-out/fan-in pipeline
- [ ] Apply timeouts and cooperative cancellation (`select!`, `CancellationToken`) and shut a server down gracefully
- [ ] Recognise the Rust-specific hazards: blocking the runtime, guards held across `.await`, deadlocks, forgotten tasks

## 1. Concept Overview
Concurrency means making progress on several things at once. A **thread** is scheduled by the OS (good for CPU-bound or blocking work); an **async task** is a state machine scheduled by a runtime on a few threads (good for many mostly-waiting requests). Tasks that touch the same data need either **shared state** with synchronisation (locks, atomics) or **message passing** (channels, one owner of the data). Real servers also need **backpressure** (bounded queues), **cancellation** (timeouts, client disconnects, shutdown) and a way to **bound parallelism**.

```
requests -> bounded queue -> N workers -> results (fan-in)      shutdown: close queue, drain, join
```

## 2. How Rust Approaches It
- **Data races are compile errors.** `Send` ("may move to another thread") and `Sync` ("may be shared by reference") are checked by the compiler. `Rc<T>` or a bare `RefCell` crossing a thread boundary simply does not compile. *Race conditions* (wrong logic) and deadlocks are still possible; there is no runtime race detector to run (unlike Go's `-race`).
- **`std::thread`** (+ `thread::scope` to borrow locals), **`std::sync`**: `Arc`, `Mutex`, `RwLock`, `mpsc`, atomics.
- **Async is opt-in and runtime-based:** `async fn` returns a lazy future; `tokio` (PROFILE choice) provides the runtime, `spawn`, `JoinSet`, `select!`, `sync::{mpsc, oneshot, Semaphore, Mutex}`, `time`. `tokio-util`'s `CancellationToken` and `futures`' `stream` helpers (`buffered`) complete the toolbox.
- **Cancellation = dropping a future.** A future that is dropped stops at its last `.await` and drops its locals. Abort a task with `JoinHandle::abort`; a dropped `JoinSet` aborts its tasks, a dropped `JoinHandle` merely detaches.
- **Spawned tasks need `Send + 'static`:** the data must be owned (move it or `Arc` it), and nothing non-`Send` may live across an `.await`.
- Idiom: prefer message passing/ownership (an actor, 3.5) or a short-lived lock; avoid `Arc<Mutex<_>>` soup.

## 3. Guided Examples

### 3.1 Threads, locks, atomics, channels
- **Goal:** the `std` building blocks.
- **Run:** `cargo run -p m06-concurrency --example threads_and_channels`
- **Code:** [examples/threads_and_channels.rs](examples/threads_and_channels.rs)
```rust
let counter = Arc::new(Mutex::new(0_u32));
let counter = Arc::clone(&counter);                 // one clone per thread
thread::spawn(move || { *counter.lock().unwrap() += 1; });
```
- **Walkthrough:** `join` returns the closure's value (or `Err` if it panicked). `thread::scope` lets threads borrow `left`/`right`. The `mpsc` loop ends only when *every* sender is dropped, hence `drop(tx)` after cloning for the producers. Sums are printed after sorting because thread order is not deterministic.
- **Expected output:**
  ```
  sum from a thread: 55
  borrowed halves: 6 + 15
  mutex counter: 4000
  atomic counter: 4000
  received: [0, 1, 10, 11, 20, 21]
  panicked thread: is_err=true
  ```

### 3.2 Async tasks on Tokio
- **Goal:** `spawn`, `join!`, `JoinSet`, shared state, `spawn_blocking`.
- **Run:** `cargo run -p m06-concurrency --example async_tasks`
- **Code:** [examples/async_tasks.rs](examples/async_tasks.rs)
```rust
let primes = tokio::task::spawn_blocking(|| count_primes(50_000)).await.unwrap();
```
- **Walkthrough:** `join!` polls both futures in one task; `JoinSet::join_next` yields results in *completion* order (fast, medium, slow). The std `Mutex` guard is dropped at the end of `push(..);`, before any `.await`. CPU-heavy or blocking code goes to `spawn_blocking` so it cannot stall the runtime threads. A panicking task becomes a `JoinError`.
- **Expected output:**
  ```
  spawned after 5ms
  join!: a after 20ms, b after 5ms
  completed: fast after 10ms
  completed: medium after 35ms
  completed: slow after 60ms
  log: [0, 1, 2, 3, 4]
  primes below 50000: 5133
  panicked task: is_panic=true
  ```

### 3.3 Worker pool, backpressure, bounded concurrency
- **Goal:** fan-out/fan-in and limits.
- **Run:** `cargo run -p m06-concurrency --example worker_pool`
- **Code:** [examples/worker_pool.rs](examples/worker_pool.rs)
```rust
let job = jobs_rx.lock().await.recv().await;      // lock only while waiting for a job
let _permit = limit.acquire().await.unwrap();      // Semaphore: at most 2 tasks inside
```
- **Walkthrough:** a bounded channel of capacity 2 rejects the third `try_send` (`send().await` would wait instead: that is backpressure). Three workers share one receiver; results fan in through a second channel that closes when all workers finish. The `Semaphore` version limits parallelism without a pool: 6 tasks, 2 permits, peak 2.
- **Expected output:**
  ```
  third try_send: Err("no available capacity")
  after one recv: Ok(())
  squares: [1, 4, 9, 16, 25, 36, 49, 64]
  6 tasks, permits=2, peak in flight: 2
  ```

### 3.4 Timeouts and cancellation
- **Goal:** stop work you no longer need.
- **Run:** `cargo run -p m06-concurrency --example cancellation`
- **Code:** [examples/cancellation.rs](examples/cancellation.rs)
```rust
tokio::select! {
    _ = token.cancelled() => break,
    item = rx.recv() => { /* handle item */ }
}
```
- **Walkthrough:** `timeout` drops the inner future at the deadline. `select!` drops the losing branches (so every branch must be *cancel-safe*: `mpsc::Receiver::recv` is, a half-finished read loop is not). `abort()` cancels at the next `.await` and runs destructors (`cleanup: aborted task`). `CancellationToken` is cooperative: the worker chooses where to stop; child tokens are cancelled by the parent, not vice versa.
- **Expected output:**
  ```
  timeout: Err("deadline has elapsed")
  timeout: Ok(42)
  select: fast branch
  cleanup: aborted task
  abort: is_cancelled=true
  worker handled 1
  worker handled 2
  worker handled 3
  worker stopped after 3 items
  child cancelled, parent cancelled? false
  parent cancelled, child cancelled? true
  ```

### 3.5 Task Tracker API with an actor and an atomic counter
- **Goal:** shared state through message passing, plus graceful shutdown.
- **Run:** `ADDR=127.0.0.1:8080 cargo run -p m06-concurrency --example task_actor_api` (Ctrl-C or SIGTERM: stops accepting, then prints `stopped cleanly` on stderr)
- **Code:** [examples/task_actor_api.rs](examples/task_actor_api.rs)
```rust
enum Command { Create { title: String, reply: oneshot::Sender<Task> }, List { reply: oneshot::Sender<Vec<Task>> } }
```
- **Walkthrough:** one task (`run_actor`) owns the `Vec<Task>`; handlers send a `Command` with a `oneshot` reply channel, so there is no lock and no shared mutable data. The bounded command channel gives backpressure. `requests` is an `AtomicU64` bumped by a `from_fn` middleware (a counter needs no actor). A `CancellationToken` ties signal handling to `with_graceful_shutdown`; when the router is dropped the actor's channel closes and the actor exits. `created_at` is a fixed value so the output is reproducible.
- **Expected output** (a fresh server; `requests` counts every request including `/stats`):
  ```
  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"Write docs"}'
  HTTP/1.1 201 Created
  location: /tasks/1

  {"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}

  $ curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"   "}'
  HTTP/1.1 422 Unprocessable Entity
  {"error":{"code":"validation_failed","fields":{"title":"must be 1-200 characters"},"message":"invalid task"}}

  $ curl -s -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"Review PR"}'   # task 2
  $ curl -s localhost:8080/tasks
  {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"},{"id":2,"title":"Review PR","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}],"total":2}

  $ curl -s localhost:8080/stats
  {"requests":5,"tasks":2}
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `std::thread::sleep`, heavy loops or blocking I/O inside `async fn` | the task never yields, so it freezes a runtime worker thread | `tokio::time::sleep`, `tokio::fs`, or `spawn_blocking` for CPU/blocking work |
| "future cannot be sent between threads safely" | a `std::sync::MutexGuard`, `Rc` or `RefCell` borrow lives across `.await` | end the guard's scope before the `.await`; clone data out; only use `tokio::sync::Mutex` if you must hold the lock across an await |
| Receiver loop never ends | one `Sender` clone is still alive (often the original) | `drop(tx)` after cloning; `rx.recv()` returns `None` only when all senders are gone |
| Work silently stops or is never awaited | dropped `JoinHandle` detaches; dropped `JoinSet` aborts; panics hide in unawaited handles | keep handles, `join_next()` them, handle `JoinError` |
| Deadlock with two locks | thread A holds `a` wants `b`, thread B holds `b` wants `a` | one global lock order, or one lock (or an actor) for both |
| Unbounded fan-out (`spawn` per item) overwhelms a downstream | nothing limits parallelism | `Semaphore`, `buffered(n)`, or a fixed worker pool + bounded channel |
| `select!` loses data | a branch future was cancelled half way (not cancel-safe) | use cancel-safe calls (`recv`, `Notify::notified`) or keep the future outside the loop and `&mut` it |
| `tokio::spawn(async { use_local(&x) })` fails with "borrowed value does not live long enough" | spawned futures must be `'static` | move or `Arc`-clone the data; `thread::scope` for threads that borrow |
| Counters with `Relaxed` used to *publish* other data | `Relaxed` orders nothing but the counter itself | plain counters: `Relaxed` is fine; for hand-offs use a channel or `Acquire`/`Release` |

## 5. Coming From Other Languages
- **Go:** goroutine ~ task (`tokio::spawn`), `chan` ~ `mpsc` (bounded = buffered), `select` ~ `tokio::select!`, `context.Context` ~ `CancellationToken` + dropping futures, `errgroup`/`WaitGroup` ~ `JoinSet`. Rust forbids data races at compile time; Go finds them with `-race`. Async functions do nothing until polled/awaited.
- **C++:** RAII guards (`lock()` returns a guard); `Send`/`Sync` encode what `std::thread` leaves to you.
- **JavaScript/Python:** `async`/`await` looks the same, but futures are lazy, run on a multi-threaded runtime (tasks may move between threads), and cancellation is by drop, not an exception.
- **Java/C#:** `Task`/`CompletableFuture` start eagerly and use `CancellationToken` explicitly; here, dropping the future is the default cancel.

## 6. Exercises
Crates in [exercises/](exercises/), reference solutions in [solutions/](solutions/). Tests are deterministic (no timing assumptions beyond a few milliseconds; hangs are guarded by timeouts).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Shared metrics | Easy | `Arc`, `Mutex`, atomics, `thread::spawn`/`join` | `cargo test -p m06-ex1-shared-metrics` |
| 2 | Bounded fan-out | Medium | `futures::stream` (`buffered`, `buffer_unordered`), cancel by drop | `cargo test -p m06-ex2-bounded-fanout` |
| 3 | Worker pool | Stretch | bounded `mpsc`, shared receiver, graceful shutdown | `cargo test -p m06-ex3-worker-pool` |

**Exercise 1 - Shared metrics.** *Task:* implement `Metrics::record(&self, status)`, `snapshot`, and `record_in_parallel` (one thread per batch, join all). *Acceptance:* `tests/metrics.rs` (8 threads x 1000 updates lose nothing; `Metrics` is `Send + Sync`). *Hint:* atomics for the counters, a `Mutex<BTreeMap>` for the per-status map; clone the `Arc` *before* `move`.

**Exercise 2 - Bounded fan-out.** *Task:* `map_concurrent` (ordered, at most `limit` in flight) and `map_concurrent_fail_fast` (first error wins; the rest are cancelled by dropping). *Acceptance:* `tests/fanout.rs`, including a call counter that proves no more than `limit` run at once and a drop counter that proves cancellation. *Hint:* `stream::iter(items).map(f).buffered(n)`; for fail-fast use `buffer_unordered` over `(index, future)` and put results back by index.

**Exercise 3 - Worker pool.** *Task:* `WorkerPool::new/submit/shutdown`: fixed workers, a bounded queue, `submit` returns `Full` instead of waiting, `shutdown` drains accepted jobs. *Acceptance:* `tests/pool.rs` (a barrier proves real parallelism; a gated handler proves rejection and draining). *Hint:* tokio's `Receiver` has one consumer: share it as `Arc<tokio::sync::Mutex<Receiver<J>>>` and lock only while waiting; dropping the `Sender` ends the workers' loops.

## 7. Recap
- The compiler prevents data races (`Send`/`Sync`); you still design against deadlocks, blocking, and unbounded growth.
- Async tasks for many waiting requests, `spawn_blocking`/threads for CPU or blocking work.
- Prefer ownership + channels (or one short lock); bound every queue and every fan-out.
- Cancellation is dropping a future; make shutdown explicit with a token and drain what you accepted.
- When to reach for what: counter -> atomic; small shared map -> `Mutex`; state with logic -> actor/channel; many calls -> `JoinSet`/`buffered` + `Semaphore`.

## 8. Self-check Questions
1. Why does `tokio::spawn(async { let g = std_mutex.lock().unwrap(); work().await; })` fail to compile, and what are two fixes?
2. A worker loop `while let Some(job) = rx.recv().await` never finishes at shutdown. List two reasons and how you would find them.
3. What exactly happens to a future's local variables when `timeout` fires?
4. You need "at most 5 upstream calls at once, results in request order": which two tools would you combine?

## 9. Further Reading
- The Tokio tutorial: <https://tokio.rs/tokio/tutorial> (shared state, channels, select, graceful shutdown)
- *The Rust Book*, ch. 16 "Fearless Concurrency" and ch. 17 "Async": <https://doc.rust-lang.org/book/>
- Alice Ryhl, "Actors with Tokio": <https://ryhl.io/blog/actors-with-tokio/>

---
**Next:** [07 HTTP Client](../07-http-client/README.md)
