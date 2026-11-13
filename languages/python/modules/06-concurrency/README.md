# 06. Concurrency in Python

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01 Language Fundamentals, 05 Database Access

## Learning Outcomes
After this module you can:
- [ ] Choose between threads, `asyncio` and processes, and explain what the GIL changes.
- [ ] Protect shared state with locks, and prefer message passing (queues) where possible.
- [ ] Run work concurrently with `ThreadPoolExecutor`, `asyncio.gather` and `TaskGroup`.
- [ ] Build a worker pool with a bounded queue (back-pressure) and limit concurrency with a semaphore.
- [ ] Apply timeouts and cancellation correctly, with cleanup.
- [ ] Propagate a request id through concurrent tasks with `contextvars`, and recognise races and deadlocks.

## 1. Concept Overview
Concurrency means making progress on several things at once: waiting for the database while serving another request. Two models dominate: **threads** (the OS schedules them, any line can be interrupted, shared memory needs locks) and an **event loop** (one thread, tasks switch only at `await`, so most shared-state races disappear but one blocking call stalls everyone).

```
event loop thread:  task A ──await──┐   task B runs ──await──┐   task A resumes ...
worker threads:     blocking calls (to_thread / executor) report back to the loop
```

Real programs need four tools: **limits** (worker pools, semaphores, bounded queues so a fast producer cannot flood a slow consumer), **cancellation and deadlines** (every outbound wait has a timeout), **safe sharing** (locks, or better, hand data over via queues), and **context propagation** (request id, deadline). The classic failures are race conditions (outcome depends on timing), deadlocks (two parties wait for each other) and leaked work (tasks nobody waits for).

## 2. How Python Approaches It
- **The GIL:** in the standard CPython build only one thread runs Python bytecode at a time. Threads still help for I/O-bound work (the lock is released while waiting on sockets, files, `sleep`), but not for CPU-bound Python code: use `multiprocessing` / `ProcessPoolExecutor` (separate interpreters; arguments and results are pickled) for that. The GIL does **not** make compound operations such as `counter += 1` atomic.
- **`threading` + `concurrent.futures`:** `Thread`, `Lock`, `Barrier`, `ThreadPoolExecutor` (`submit`, `map`). Use the executor, not raw threads, for pools.
- **`asyncio`:** `async`/`await`, `asyncio.run`, `TaskGroup` (structured concurrency, 3.11+), `gather`, `timeout()`, `Queue` (with `shutdown()`, 3.13+), `Semaphore`, `Lock`, `to_thread`. Libraries must be async-aware (e.g. `httpx.AsyncClient`, see [module 07](../07-http-client/README.md)); FastAPI runs `async def` handlers on the loop and plain `def` handlers in a thread pool.
- **`contextvars`:** per-task state; a task inherits a *copy* of the context at creation, and `to_thread` copies it as well.
- Third-party: none. Non-idiomatic: `threading.Thread` for everything, `time.sleep` in coroutines, bare `asyncio.create_task` without keeping or awaiting the task.

| Workload | Reach for |
|----------|-----------|
| many network/DB waits, async libraries available | `asyncio` + `TaskGroup` |
| blocking library or legacy sync code | `ThreadPoolExecutor` or `asyncio.to_thread` |
| CPU-bound Python code | `ProcessPoolExecutor` |

## 3. Guided Examples

### 3.1 Threads, locks and a deadlock
- **Goal:** see why shared mutable state needs a lock, and what lock-ordering deadlocks look like.
- **Run:** `python modules/06-concurrency/examples/01_threads_and_locks.py`
- **Code:** [examples/01_threads_and_locks.py](examples/01_threads_and_locks.py)
- **Walkthrough:** `self.value += 1` is read, add, write: another thread can interleave and lose an update, so the unsafe total can be lower than expected (it varies by run, so the example only prints that it never exceeds it). `with self._lock:` makes the sequence atomic. `pool.map` keeps input order. In the deadlock demo each thread holds one lock and wants the other; `acquire(timeout=...)` lets the demo end. In real code take locks in one global order.
- **Expected output:**

  ```
  locked == expected: True
  unsafe <= expected: True
  [0, 1, 4, 9, 16, 25, 36, 49]
  ['t1: gave up (would deadlock)', 't2: gave up (would deadlock)']
  ```

### 3.2 asyncio basics
- **Goal:** coroutines, overlapping waits, `TaskGroup`, background tasks and the blocking-call trap.
- **Run:** `python modules/06-concurrency/examples/02_asyncio_basics.py`
- **Code:** [examples/02_asyncio_basics.py](examples/02_asyncio_basics.py)
- **Walkthrough:** calling `fetch(...)` only creates a coroutine; nothing runs until it is awaited or wrapped in a task. Three sequential 50 ms awaits take 150 ms, `gather` takes about 50 ms. The `TaskGroup` block ends only when all its tasks have. A background task is stored in a set because the loop keeps only a weak reference. A ticker task counts loop iterations: during `time.sleep` it cannot run at all, during `await asyncio.to_thread(time.sleep, ...)` it keeps going.

```python
async with asyncio.TaskGroup() as group:
    slow = group.create_task(fetch("slow", 0.03))
    fast = group.create_task(fetch("fast", 0.01))
print(fast.result(), "/", slow.result())
```

- **Expected output:**

  ```
  coroutine
  ['a done', 'b done', 'c done'] | overlapped: True
  fast done / slow done
  background done
  ticks during time.sleep: 0
  ticks during to_thread: several
  ```

### 3.3 Cancellation and timeouts
- **Goal:** deadlines that really stop work, and failure handling in a `TaskGroup`.
- **Run:** `python modules/06-concurrency/examples/03_cancellation_and_timeouts.py`
- **Code:** [examples/03_cancellation_and_timeouts.py](examples/03_cancellation_and_timeouts.py)
- **Walkthrough:** `asyncio.timeout()` cancels the body at the deadline and raises `TimeoutError` outside it. Cancellation is an exception (`CancelledError`) thrown at the current `await`: `finally` blocks run, and `except CancelledError` must **re-raise**. When one task in a `TaskGroup` fails, its siblings are cancelled and the failures surface as an `ExceptionGroup`, caught with `except*`.
- **Expected output:**

  ```
  timed out
  ['timed: started', 'timed: cancelled', 'timed: cleanup']
  task cancelled: True
  ['manual: started', 'manual: cancelled', 'manual: cleanup']
  ValueError handled: ['bad input']
  ['sibling: started', 'sibling: cancelled', 'sibling: cleanup']
  ```

### 3.4 Worker pool, semaphore and fan-in
- **Goal:** bounded queues, limited concurrency and results as they complete.
- **Run:** `python modules/06-concurrency/examples/04_worker_pool_and_queues.py`
- **Code:** [examples/04_worker_pool_and_queues.py](examples/04_worker_pool_and_queues.py)
- **Walkthrough:** `Queue(maxsize=2)` makes `put` wait while the queue is full (back-pressure). `queue.shutdown()` after the last item lets workers drain the queue and then leave through `QueueShutDown`. A `Semaphore(2)` caps in-flight calls. `asyncio.as_completed` yields results in completion order.
- **Expected output:**

  ```
  [(0, 0), (1, 1), (2, 4), (3, 9)] ... total 12
  max busy workers: 3 | queue limit respected: True
  [0, 10, 20, 30, 40, 50] | peak in flight: 2
  completion order: ['fast', 'medium', 'slow']
  ```

### 3.5 Task Tracker bulk import
- **Goal:** combine a semaphore, per-item timeouts, a lock and `contextvars` in one realistic flow.
- **Run:** `python modules/06-concurrency/examples/05_task_import.py`
- **Code:** [examples/05_task_import.py](examples/05_task_import.py)
- **Walkthrough:** each item validates, stores (an `asyncio.Lock` makes "read id, suspend, write id" one step) and notifies the owner under a 50 ms `asyncio.timeout`. Every item returns an `Outcome` instead of raising, so one bad item does not cancel the rest. `request_id` is a `ContextVar`: each request runs in its own task, and the tasks created inside inherit its value without passing it around.
- **Expected output:**

  ```
  0 True [req-1] task 1: Write docs
  1 False [req-1] rejected: title must be 1-200 characters
  2 False [req-1] notification timed out: slow report
  3 True [req-1] task 3: Deploy
  second request ids: ['req-2']
  outside any request: -
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `time.sleep()`, `requests.get()` or a DB call inside `async def` | it blocks the single event-loop thread; every request stalls | `await asyncio.to_thread(...)`, an async library, or a plain `def` handler |
| Calling `fetch()` without `await` | the call returns a coroutine object that never runs (`RuntimeWarning: coroutine ... was never awaited`) | `await` it or wrap it in `create_task` / a `TaskGroup` |
| `asyncio.create_task(...)` result thrown away | the loop holds only a weak reference: the task can be garbage-collected mid-flight and its exception is lost | use a `TaskGroup`, or keep tasks in a set and discard on completion |
| Threads for CPU-bound work | the GIL runs one thread of Python bytecode at a time | `ProcessPoolExecutor`, or move the hot loop out of Python |
| "The GIL makes it thread-safe" | `x += 1`, check-then-insert and read-modify-write are several bytecodes | `threading.Lock`, or hand data over with a `Queue` |
| Swallowing `CancelledError` (`except Exception`/bare `except`, or no re-raise) | cancellation is delivered as an exception | catch narrowly, always `raise` again after cleanup |
| `asyncio.run()` inside a running loop (notebooks, handlers) | `RuntimeError: asyncio.run() cannot be called from a running event loop` | `await` the coroutine; `asyncio.run` only at the program entry point |
| Unbounded `gather(*[...100_000 coroutines])` or unbounded queues | everything starts at once, memory and downstream load explode | `Semaphore`, workers + `Queue(maxsize=...)` |
| Holding a `threading.Lock` across an `await` | the lock blocks the loop thread, other tasks needing it deadlock | use `asyncio.Lock` inside coroutines, and keep critical sections short |
| Lock-order deadlock | thread 1 takes A then B, thread 2 takes B then A | one global lock order, or `acquire(timeout=...)` and back off |

Debug aids: `python -X dev` (or `asyncio.run(main(), debug=True)`) reports never-awaited coroutines and slow callbacks; `faulthandler.dump_traceback_later()` shows where a hung program waits. Python has no data-race detector like Go's `-race`, so rely on tests that stress shared state with many threads (see the exercises).

## 5. Coming From Other Languages
- **Go:** goroutines + channels map to tasks + `asyncio.Queue`; `context.Context` cancellation/deadlines map to task cancellation and `asyncio.timeout`, and values to `contextvars`. Unlike goroutines, tasks are cancelled by exception and must be awaited; a blocking call in a coroutine is far more harmful than in Go.
- **JavaScript/TypeScript:** the same single-threaded event loop and `async`/`await`, but a Python coroutine is lazy (not started until awaited or scheduled) and `Promise.all` corresponds to `gather`/`TaskGroup`.
- **Java/C#:** `ExecutorService` is `ThreadPoolExecutor`; `CompletableFuture`/`Task` are `asyncio.Task`; `CancellationToken` is built into task cancellation.
- **Rust:** `tokio::select!`/`JoinSet` resemble `timeout`/`TaskGroup`; the borrow checker prevents data races that Python leaves to you.

## 6. Exercises
Each exercise has starter code and tests in [exercises/](exercises/). Solutions are in [solutions/](solutions/). Run all commands from `languages/python/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Create-once cache | Easy | locks, check-then-act race | `pytest modules/06-concurrency/exercises/ex1_once_cache` |
| 2 | Limited gather | Medium | `Semaphore`, `TaskGroup`, ordered results | `pytest modules/06-concurrency/exercises/ex2_limited_gather` |
| 3 | Bounded pipeline | Stretch | `Queue`, back-pressure, cancellation, timeouts | `pytest modules/06-concurrency/exercises/ex3_pipeline` |

### Exercise 1: Create-once cache
- **Task:** implement `OnceCache.get_or_create` in [m06_once_cache.py](exercises/ex1_once_cache/m06_once_cache.py).
- **Acceptance:** [test_m06_once_cache.py](exercises/ex1_once_cache/test_m06_once_cache.py): 16 threads released together by a barrier trigger exactly one factory call per key; a failing factory caches nothing.
- **Hint:** the check ("is it there?") and the act ("create it") must happen under the same lock.

### Exercise 2: Limited gather
- **Task:** implement `gather_limited` in [m06_limited_gather.py](exercises/ex2_limited_gather/m06_limited_gather.py).
- **Acceptance:** [test_m06_limited_gather.py](exercises/ex2_limited_gather/test_m06_limited_gather.py): peak concurrency equals the limit, results keep input order, a failure cancels running jobs and surfaces as an `ExceptionGroup`, no task leaks.
- **Hint:** take the semaphore inside each task *before* calling the factory, and create the tasks in a `TaskGroup`.

### Exercise 3: Bounded pipeline
- **Task:** implement `run_pipeline` in [m06_pipeline.py](exercises/ex3_pipeline/m06_pipeline.py): producer, bounded queue, N workers, ordered results, optional overall timeout.
- **Acceptance:** [test_m06_pipeline.py](exercises/ex3_pipeline/test_m06_pipeline.py): the producer is throttled by the queue size, an error or timeout cancels every task, nothing is left running.
- **Hint:** wrap the `TaskGroup` in `asyncio.timeout(timeout)` (`None` means no deadline) and finish the input with `queue.shutdown()`.

## 7. Recap
- The GIL serialises Python bytecode, not your logic: shared mutable state still needs locks or queues.
- Event loop tasks switch only at `await`: never block the loop, and keep references to (or use a `TaskGroup` for) every task.
- Bound everything: workers, queue sizes, concurrency, and wait time (`asyncio.timeout`).
- Cancellation is an exception: clean up in `finally`, re-raise `CancelledError`.
- Rule of thumb: `asyncio` for many waits, threads for blocking libraries, processes for CPU.

## 8. Self-check Questions
1. Why can `counter += 1` lose updates even though CPython has a GIL?
2. A FastAPI `async def` handler calls `requests.get(...)`. What happens to other requests, and what are two fixes?
3. What does a `TaskGroup` do when one of its tasks raises, and why is that better than `gather` with its default settings?
4. Why does a bounded queue improve a pipeline where the producer is faster than the consumers?

## 9. Further Reading
- [asyncio documentation (Runner, TaskGroup, Queue, timeouts)](https://docs.python.org/3/library/asyncio.html)
- [threading](https://docs.python.org/3/library/threading.html) and [concurrent.futures](https://docs.python.org/3/library/concurrent.futures.html)
- [contextvars](https://docs.python.org/3/library/contextvars.html)
- [Python and the GIL (glossary)](https://docs.python.org/3/glossary.html#term-global-interpreter-lock)

---
**Next:** [07 HTTP Client](../07-http-client/README.md)
