# 06. Concurrency in TypeScript

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01 (language fundamentals, promises), 02 (HTTP server)

## Learning Outcomes

After this module you can:

- [ ] Explain the event loop: call stack, microtasks, timers and I/O callbacks, and predict their order
- [ ] Run async work concurrently (`Promise.all/allSettled/race/any`) instead of accidentally one after another
- [ ] Limit concurrency with a hand-written pool and build a worker pool with a bounded queue
- [ ] Cancel and time-limit work with `AbortController`, `AbortSignal.timeout` and `AbortSignal.any`
- [ ] Recognise and avoid races across `await`, unhandled rejections, and CPU-bound code blocking the loop (use `worker_threads`)

## 1. Concept Overview

Concurrency means making progress on several things during the same period; parallelism means executing them at the same instant. A backend spends most of its time _waiting_ (database, network, disk), so the useful tool is overlapping waits. CPU-heavy work needs real parallelism: another thread or process.

Every concurrency design answers four questions: how work is **started** (tasks, threads), how it is **limited** (pools, queues, back-pressure), how it is **stopped** (cancellation, timeouts) and how shared state is **protected** (locks, message passing, or no sharing at all).

## 2. How TypeScript Approaches It

- **One thread, an event loop.** JavaScript runs one call stack. `await` suspends a function and lets the loop run other work; nothing runs in parallel in your code, so there are no data races on memory, but there are still _logical_ races across `await`.
- **Order of execution:** synchronous code, then the microtask queue (promise reactions, `queueMicrotask`), then timers, I/O callbacks and `setImmediate`. `process.nextTick` is Node-specific and runs before promise microtasks in CommonJS, but after them at the top level of an ES module.
- **Promises are the unit of work:** a promise starts working when it is _created_, not when it is awaited. `Promise.all` (fail-fast), `allSettled` (collect everything), `race` (first to settle), `any` (first to succeed).
- **Cancellation is cooperative:** `AbortController` / `AbortSignal` is the one standard mechanism, accepted by `fetch`, `node:timers/promises`, streams and `node:fs`. A promise itself cannot be cancelled.
- **Parallelism:** `node:worker_threads` for CPU-bound work (data is copied between threads, `SharedArrayBuffer` + `Atomics` is the shared-memory exception). No locks are needed for ordinary code.
- No third-party library is used in this module ([PROFILE.md](../../PROFILE.md)).

| Need                            | Tool                                                |
| ------------------------------- | --------------------------------------------------- |
| Run N independent calls at once | `Promise.all(items.map(fn))`                        |
| At most K at once               | a pool of K runners sharing a cursor (`mapLimit`)   |
| Tolerate failures               | `Promise.allSettled`                                |
| Timeout / deadline              | `AbortSignal.timeout(ms)`, `AbortSignal.any([...])` |
| CPU-bound work                  | `worker_threads`                                    |
| Bounded background work         | queue + fixed workers + `AbortSignal`               |

## 3. Guided Examples

### 3.1 The event loop order

- **Goal:** predict the order of sync code, microtasks, `nextTick`, timers and `setImmediate`.
- **Run:** `node modules/06-concurrency/examples/01_event_loop_order.ts`
- **Code:** [01_event_loop_order.ts](examples/01_event_loop_order.ts)
- **Walkthrough:** everything is logged into an array and printed at the end. The async function body runs synchronously until its first `await`; promise reactions run next; then `nextTick` (after them, because an ES module's top level is itself a promise job); then the timer. Inside an I/O callback, `setImmediate` always beats a 0 ms timer.
- **Expected output:**

  ```
  sync: start
  sync: async function body runs until its first await
  sync: end
  microtask: promise.then
  microtask: queueMicrotask
  microtask: code after await
  nextTick
  macrotask: setTimeout 0
  setImmediate inside I/O callback
  timer inside I/O callback
  ```

### 3.2 `await` in a loop vs combinators

- **Goal:** overlap waits, pick the right combinator, see a race across `await`.
- **Run:** `node modules/06-concurrency/examples/02_await_and_combinators.ts`
- **Code:** [02_await_and_combinators.ts](examples/02_await_and_combinators.ts)

  ```ts
  for (const id of ids) out.push(await fetchTitle(id, 50)); // serial: ~4 x 50 ms
  const all = await Promise.all(ids.map((id) => fetchTitle(id, 50))); // overlapped: ~50 ms
  ```

- **Walkthrough:** timings are printed as coarse buckets to stay deterministic. `Promise.all` rejects with the first failure while the other calls keep running; `allSettled` reports each outcome. The last two blocks show a started-but-not-awaited promise and a lost update: three `unsafeIncrement()` calls read `0`, sleep, and all write `1`.
- **Expected output:**

  ```
  await in a loop: slow (~sum of the waits)
  Promise.all: fast (~longest wait)
  true [ 'task 1', 'task 2', 'task 4', 'task 5' ]
  Promise.all rejected: task 3 failed
  [ 'task 1', 'rejected: task 3 failed', 'task 2' ]
  race: task 2
  any: task 2
  finished right after starting: false
  finished after awaiting: true
  counter after 3 unsafe increments: 1
  counter after 3 safe increments: 3
  ```

### 3.3 Limiting concurrency and cancelling

- **Goal:** a hand-written `mapLimit`, plus `AbortController`, `AbortSignal.timeout` and `AbortSignal.any`.
- **Run:** `node modules/06-concurrency/examples/03_limit_and_cancel.ts`
- **Code:** [03_limit_and_cancel.ts](examples/03_limit_and_cancel.ts)

  ```ts
  const runner = async () => {
    while (next < items.length) {
      const index = next++; // safe: one thread, no await between check and increment
      results[index] = await fn(items[index] as T, index);
    }
  };
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, runner));
  ```

- **Walkthrough:** `limit` runners share one cursor, so at most `limit` calls are in flight and results keep input order. `sleep(ms, undefined, { signal })` rejects with an `AbortError` as soon as the signal aborts; for a timeout signal the `TimeoutError` is in `err.cause`. A loop with no await point must call `signal.throwIfAborted()`. The last block shows that this `mapLimit` has no fail-fast cancellation: remaining items still start after the first error (exercise 2 fixes that).
- **Expected output:**

  ```
  2,4,6,8,10,12,14,16 peak in flight: 3
  cancelled: AbortError
  timed out: AbortError <- TimeoutError
  combined aborted: true - user left
  loop stopped: AbortError
  rejected: boom
  items started anyway: [ 1, 2, 3, 4, 5, 6 ]
  ```

### 3.4 CPU-bound work and worker threads

- **Goal:** see the event loop starve, then move the work to a `Worker`.
- **Run:** `node modules/06-concurrency/examples/04_worker_threads.ts`
- **Code:** [04_worker_threads.ts](examples/04_worker_threads.ts)
- **Walkthrough:** a 1 ms `setInterval` heartbeat counts how often the loop was free. A synchronous `fib(36)` on the main thread starves it; the same computation in a `Worker` (the same file, branch on `isMainThread`) leaves it running. `workerData` goes in, `postMessage` brings the result back; errors in the worker arrive on the `error` event. Spawning a worker costs tens of milliseconds, so real services keep a pool.
- **Expected output:**

  ```
  main thread: 14930352 heartbeat ticks: almost none
  worker thread: 14930352 heartbeat ticks: many
  ```

### 3.5 A bounded job pool

- **Goal:** Task Tracker background jobs: fixed workers, a bounded queue with back-pressure, cancellation and graceful close.
- **Run:** `node modules/06-concurrency/examples/05_job_pool.ts`
- **Code:** [05_job_pool.ts](examples/05_job_pool.ts)
- **Walkthrough:** `submit` hands a job to an idle worker or queues it, and answers `queue_full` (an API would return `429`) when no slot is left. Idle workers park on a promise; `close()` wakes them, lets queued jobs finish, then resolves. `cancel` removes a queued job or aborts the `AbortController` of a running one. The pool's state is only touched from synchronous code, so it needs no locks.
- **Expected output:**

  ```
  accepted job 1
  accepted job 2
  accepted job 3
  accepted job 4
  rejected: queue_full
  cancel queued job 3: true
  cancel running job 1: true
  1:cancelled 2:running 3:cancelled 4:running
  after close: { ok: false, reason: 'closed' }
  1 sleep   cancelled
  2 sleep   succeeded
  3 sleep   cancelled
  4 sleep   succeeded
  5 reverse succeeded rekcart
  6 fail    failed    this job always fails
  ```

## 4. Common Pitfalls

| Mistake                                               | Why it happens                                                                           | Fix                                                                                                          |
| ----------------------------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `for (...) await call()` over independent calls       | `await` suspends the loop body, so calls run one by one                                  | start all, then `Promise.all`, or `mapLimit` when the count is large                                         |
| `items.forEach(async (x) => ...)`                     | `forEach` ignores the returned promises: nothing waits, errors are unhandled             | `await Promise.all(items.map(async ...))` or a `for...of` loop                                               |
| Forgotten `await`                                     | the call returns a promise; the function moves on and the work (and its failure) escapes | `await` it, or `void`/`.catch()` on purpose; enable a no-floating-promises lint rule                         |
| Unhandled rejection crashes the process               | Node 15+ terminates on an unhandled rejection                                            | attach a handler to every promise you start; log in `process.on('unhandledRejection')` as a last resort only |
| `Promise.all` leaves siblings running after one fails | it only rejects early; promises cannot be cancelled                                      | pass an `AbortSignal` to the work and abort it when one fails                                                |
| Unbounded `Promise.all(10_000 calls)`                 | every call starts at once: sockets, memory, upstream limits                              | `mapLimit` or a pool with a bounded queue                                                                    |
| Lost updates across `await`                           | read, `await`, write interleaves with other callers                                      | do read-modify-write synchronously, or serialise with a queue                                                |
| CPU-heavy loop in a handler                           | one thread: nothing else (timers, other requests) runs meanwhile                         | `worker_threads`, or split into chunks that yield with `await setImmediate`                                  |
| Timer keeps the process alive / never cleaned         | pending `setTimeout` handles stop Node from exiting                                      | `AbortController` to cancel, `timer.unref()` for background timers, clear in `finally`                       |
| `setTimeout(async () => ...)` swallowing errors       | the timer ignores the returned promise                                                   | catch inside, or use `await sleep()` from `node:timers/promises`                                             |

## 5. Coming From Other Languages

- **JavaScript:** identical model; TypeScript only adds `Promise<T>` types, and does not warn about floating promises by default.
- **C#:** `async`/`await` is the same idea, but `Task`s run on a thread pool while Promises run on one thread. `AbortSignal` plays the role of `CancellationToken`; `mapLimit` of `SemaphoreSlim`; `worker_threads` of `Task.Run`.
- **Java:** no thread pools or `synchronized` for ordinary code; `CompletableFuture` is the closest relative of a promise, `ExecutorService` of the pool in 3.5.
- **Python:** `asyncio` is the nearest relative (`gather` is `Promise.all`), but a coroutine does nothing until awaited or scheduled, while a promise starts running when it is created.
- **Go:** no goroutines, channels or `select`; a "channel" is a queue plus promises, and `context.Context` is an `AbortSignal`. A blocking call stalls the whole loop, not just one goroutine.

## 6. Exercises

Starter code and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/).

| #   | Title                                                      | Difficulty | Skill practiced                                           | Run tests                                                                  |
| --- | ---------------------------------------------------------- | ---------- | --------------------------------------------------------- | -------------------------------------------------------------------------- |
| 1   | [Collect results](exercises/ex1_settle_all/settle-all.ts)  | Easy       | `allSettled`, `any`, timeout without leaked timers        | `node --test "modules/06-concurrency/exercises/ex1_settle_all/*.test.ts"`  |
| 2   | [Bounded `mapLimit`](exercises/ex2_map_limit/map-limit.ts) | Medium     | concurrency limit, order, fail-fast, `AbortSignal`        | `node --test "modules/06-concurrency/exercises/ex2_map_limit/*.test.ts"`   |
| 3   | [Task pool](exercises/ex3_worker_pool/task-pool.ts)        | Stretch    | workers, bounded FIFO queue, cancellation, graceful close | `node --test "modules/06-concurrency/exercises/ex3_worker_pool/*.test.ts"` |

### Exercise 1: Collect results

- **Task:** implement `collect` (start everything at once, never reject, report failures with their index), `firstSuccess` and `withTimeout` (reject with `TimeoutError`, leave no timer behind).
- **Acceptance:** all tests in `settle-all.test.ts` pass.
- **Hint:** a task that throws synchronously still has to become a rejection: call it inside an `async` arrow. A cancellable `sleep` from `node:timers/promises` makes timer cleanup easy.

### Exercise 2: Bounded `mapLimit`

- **Task:** implement `mapLimit(items, limit, fn, { signal })` with ordered results, at most `limit` calls in flight, fail-fast without starting new calls, and abort support.
- **Acceptance:** all tests in `map-limit.test.ts` pass.
- **Hint:** share a cursor between `limit` runner loops and add a `stopped` flag that every runner checks before taking the next item.

### Exercise 3: Task pool

- **Task:** implement `TaskPool` (`run`, `close`, `active`, `pending`): at most `workers` tasks run at once, up to `capacity` wait in FIFO order, more are rejected with `QueueFullError`; a queued task can be cancelled with its signal; `close()` drains.
- **Acceptance:** all tests in `task-pool.test.ts` pass.
- **Hint:** keep a queue of entries `{ start, cancel }`. Update `active` _before_ resolving the caller's promise so state is consistent when the caller resumes.

## 7. Recap

- One thread plus an event loop: overlap waits with promises, move CPU work to workers.
- Order: sync code, microtasks, then timers/I/O. A promise starts working when it is created.
- `Promise.all` is fail-fast, `allSettled` collects, `race` and `any` pick a winner; none of them cancels the losers.
- Bound everything: concurrency (`mapLimit`), queues (back-pressure) and time (`AbortSignal.timeout`).
- Cancellation is cooperative: pass a signal down and check it.

## 8. Self-check Questions

1. Why does `await` inside a `for` loop make three 100 ms calls take 300 ms, and what is the smallest change that fixes it?
2. In which order do `setTimeout(0)`, `Promise.resolve().then`, `process.nextTick` and `setImmediate` run, and why can a CommonJS script differ from an ES module?
3. `Promise.all([a(), b()])` rejects because `a()` fails. What happens to `b()`?
4. The single thread rules out data races, yet `counter` ends up `1` after three concurrent increments. Why?
5. When is `worker_threads` the right answer, and when is it overkill?

## 9. Further Reading

- Node.js docs: [The event loop, timers and `process.nextTick()`](https://nodejs.org/en/learn/asynchronous-work/event-loop-timers-and-nexttick)
- MDN: [`AbortController`](https://developer.mozilla.org/en-US/docs/Web/API/AbortController) and [Promise combinators](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Promise)
- Node.js docs: [`worker_threads`](https://nodejs.org/api/worker_threads.html)

---

**Next:** [07 HTTP Client](../07-http-client/README.md)
