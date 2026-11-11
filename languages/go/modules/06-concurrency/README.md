# 06. Concurrency in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02

## Learning Outcomes
After this module you can:
- [ ] Start goroutines, wait for them, and protect shared state with `sync.Mutex`, `sync.RWMutex` or `sync/atomic`
- [ ] Build pipelines and fan-out/fan-in with channels, respecting channel ownership (the sender closes)
- [ ] Propagate cancellation and deadlines with `context.Context`
- [ ] Bound concurrency and collect errors with `errgroup`, keeping results in order
- [ ] Find races with `go test -race` and recognise goroutine leaks and deadlocks

## 1. Concept Overview
Concurrency is structuring a program as independently progressing tasks; it pays off in a backend because most time is spent waiting on the network and the database. The hard parts are the same in every language: **shared mutable state** (races), **lifecycle** (who stops a task, who waits for it) and **backpressure** (how many tasks may run at once). Two answers exist: share memory guarded by locks, or pass ownership of data through channels.

```
producer --chan--> stage --chan--> stage --chan--> consumer
   (closes its output)                       ctx.Done() stops all of them
```

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| Run concurrently | `go f()`: goroutines are cheap (a few KB), scheduled by the runtime onto OS threads |
| Wait for them | `sync.WaitGroup` (`wg.Go(func(){...})` since Go 1.25 does `Add`/`go`/`Done`) |
| Guard shared state | `sync.Mutex` / `sync.RWMutex`, `sync/atomic` (`atomic.Int64`), `sync.Once` |
| Pass data | channels: `chan T`, direction types `<-chan T` / `chan<- T`, `select` |
| Cancel and time out | `context.WithCancel/WithTimeout/WithCancelCause`, `ctx.Done()` |
| Fan-out with errors | `golang.org/x/sync/errgroup`: `SetLimit`, first error cancels the group |
| Detect races | `go test -race`, `go run -race` (needs cgo/gcc) |

Idioms: "do not communicate by sharing memory; share memory by communicating" is a guideline, not a rule. A mutex around a map is often the simplest correct answer. Prefer **synchronous APIs** and let the caller add `go`; always give a goroutine a way to stop (context or closed channel); the **sender** owns and closes a channel; the zero value of `sync.Mutex` is ready and must never be copied.

## 3. Guided Examples

### 3.1 WaitGroup, Mutex, atomic
- **Goal:** many goroutines updating shared state correctly.
- **Run:** `go run -race ./modules/06-concurrency/examples/01_waitgroup_mutex`
- **Code:** [examples/01_waitgroup_mutex/main.go](examples/01_waitgroup_mutex/main.go)
  ```go
  for i := range workers {
      wg.Go(func() { stats.Inc(...); hits.Add(1) })
  }
  wg.Wait()
  ```
- **Walkthrough:** `Stats` keeps its mutex next to the map it guards and hands out a *copy* in `Snapshot`. `atomic.Int64` suits a single counter. Each loop iteration has its own `i` (Go 1.22+), so closures are safe; writing to distinct slice indices needs no lock. `sync.Once` runs init once.
- **Expected output:**
  ```
  even 4000
  odd 4000
  atomic total: 8000
  [0 1 4 9 16]
  init ran 1 time(s)
  ```

### 3.2 Channels and pipelines
- **Goal:** ownership, closing, `range`, `select`, fan-out/fan-in.
- **Run:** `go run ./modules/06-concurrency/examples/02_channels_pipeline`
- **Code:** [examples/02_channels_pipeline/main.go](examples/02_channels_pipeline/main.go)
  ```go
  func generate(nums ...int) <-chan int {
      out := make(chan int)
      go func() { defer close(out); for _, n := range nums { out <- n } }()
      return out
  }
  ```
- **Walkthrough:** each stage owns its output and closes it when its input ends, so `for v := range ch` terminates naturally. `merge` closes its output only after a `WaitGroup` says every sender is done. Fan-in order is not deterministic, so the example sorts before printing. A closed channel yields the zero value with `ok == false`; `select` with `default` never blocks.
- **Expected output:**
  ```
  2 2
  a b
  pipeline: [1 4 9 16]
  fan-out/fan-in: [1 4 9 16 25 36]
  closed channel ok: false
  nothing ready
  got 42
  ```

### 3.3 Context: cancel, timeout, cause
- **Goal:** stop work that nobody waits for any more.
- **Run:** `go run ./modules/06-concurrency/examples/03_context_cancel`
- **Code:** [examples/03_context_cancel/main.go](examples/03_context_cancel/main.go)
  ```go
  select {
  case <-time.After(d):  return "rows", nil
  case <-ctx.Done():     return "", ctx.Err()
  }
  ```
- **Walkthrough:** the ticker goroutine exits on `ctx.Done()` and closes `stopped`, so the caller can *wait* for the exit (cancelling is a request, not a join). `WithTimeout` yields `context.DeadlineExceeded`; always `defer cancel()` to release the timer. Children inherit cancellation from parents; `WithCancelCause` records why. `WithValue` is for request-scoped data such as a request id, with an unexported key type.
- **Expected output:**
  ```
  ticker stopped, ticked at least once: true
  timeout: context deadline exceeded true
  fast query: rows <nil>
  child: context canceled | cause: shutting down
  request id: req-123
  cleaned up
  ```

### 3.4 errgroup: bounded fan-out
- **Goal:** at most N tasks at once, ordered results, first error wins.
- **Run:** `go run ./modules/06-concurrency/examples/04_errgroup_pool`
- **Code:** [examples/04_errgroup_pool/main.go](examples/04_errgroup_pool/main.go)
  ```go
  g, ctx := errgroup.WithContext(ctx)
  g.SetLimit(limit)
  for i, id := range ids {
      g.Go(func() error { v, err := fetch(ctx, id); results[i] = v; return err })
  }
  err := g.Wait()
  ```
- **Walkthrough:** `SetLimit` makes `g.Go` block when the pool is full, which is the backpressure. Writing to `results[i]` keeps input order without a lock. The first non-nil error cancels `ctx` for the rest and is the one `Wait` returns (it still wraps correctly with `errors.Is`).
- **Expected output:**
  ```
  [item-1 item-2 item-3 item-5 item-6 item-7 item-8 item-9] <nil>
  never more than 3 in flight: true
  [] fetch 4: boom true
  cancelled: context canceled
  ```

### 3.5 Task Tracker: concurrent store and worker pool
- **Goal:** the pieces together: an `RWMutex`-guarded store, a producer that owns the job channel, a fixed pool of workers, cancellation.
- **Run:** `go run -race ./modules/06-concurrency/examples/05_task_enricher`
- **Code:** [examples/05_task_enricher/main.go](examples/05_task_enricher/main.go)
- **Walkthrough:** `Get` returns a clone of the tags so callers cannot mutate shared state. `Enrich` starts one producer (closes `jobs` when done or when `ctx` ends) and N workers ranging over `jobs`; `wg.Wait()` guarantees nothing is left running. Four readers hammer the store while the pool writes and the race detector stays silent. This is the shape reused by the job-worker capstone (P6).
- **Expected output:**
  ```
  changed: 3 err: <nil>
  1 Write docs ["docs" "p1"]
  2 Fix bug    ["bug"]
  3 Ship it    ["release"]
  4 Plan       ["a" "b"]
  5 Empty      []
  second pass changed: 0 err: <nil>
  cancelled: context canceled
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Goroutine leak | A goroutine blocks forever on a channel nobody reads/closes, or ignores `ctx` | Every goroutine selects on `ctx.Done()` or a closed channel; buffer result channels (`make(chan T, n)`) when the receiver may leave early |
| `panic: close of closed channel` / send on closed channel | Two parties think they own the channel | One owner closes; receivers never close |
| `fatal error: concurrent map writes` | Maps are not goroutine-safe; reads racing a write also fail | Mutex (or `sync.Map` only for the special cases it documents); run `-race` |
| Lost updates with `counter++` | Read-modify-write is not atomic | `atomic.Int64` or a mutex |
| Copied mutex | Passing a struct containing `sync.Mutex` by value copies the lock state; `go vet` reports `copylocks` | Use pointer receivers and `*T` |
| `wg.Add` inside the goroutine | `Wait` may run before `Add` | Add before `go`, or use `wg.Go` |
| Holding a lock across I/O or a channel send | Long critical sections block everyone, can deadlock | Copy what you need, unlock, then do the slow work |
| Unbounded `go` per request item | 100k items -> 100k goroutines and 100k connections | `errgroup.SetLimit` or a fixed worker pool |
| `time.After` in a loop / forgetting `cancel()` | Timers live until they fire | `time.NewTimer` + `Stop`, `defer cancel()` |
| Returning while workers still run | Cancelling is a request, not a join | Cancel, then `wg.Wait()`/`g.Wait()` |
| Data race on an unsynchronised result variable | Two goroutines write the same variable (e.g. `err`) | Write per-index, or send results over a channel |

## 5. Coming From Other Languages
- **Java/C#:** goroutines are not threads or `Task`s you await; there is no return value, so results flow through channels, `errgroup` or shared slots. `context.Context` is the explicit `CancellationToken`, and every blocking function takes it first.
- **Python/JS (async/await):** no coloured functions: blocking code is ordinary code and the runtime multiplexes it. There is no event loop to stall, but CPU-bound work does occupy a real thread.
- **Rust:** the borrow checker proves data-race freedom at compile time; Go relies on convention and `-race` at test time.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/). Run with `-race` as well.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Safe counter | Easy | mutex, zero value, defensive copy | `go test -race ./modules/06-concurrency/exercises/ex1_safe_counter/` |
| 2 | Bounded map | Medium | errgroup, ordering, limit, cancellation | `go test -race ./modules/06-concurrency/exercises/ex2_bounded_map/` |
| 3 | First success | Stretch | racing attempts, no leaks, `errors.Join` | `go test -race ./modules/06-concurrency/exercises/ex3_first_success/` |

### Exercise 1: Safe counter
- **Task:** `Counter` works single-threaded but is not safe: fix it for concurrent use and make `Snapshot` return a copy.
- **Acceptance:** `TestConcurrentUpdates`, `TestSnapshotIsACopy`, `TestSnapshotWhileWriting`, `TestZeroValueIsUsable` (also clean under `-race`).
- <details><summary>Hint</summary>Embed a `sync.Mutex` field next to the map, lock in every method, and clone the map with `maps.Clone` while holding the lock.</details>

### Exercise 2: Bounded map
- **Task:** implement generic `Map(ctx, limit, in, fn)`: ordered results, at most `limit` concurrent calls, first error cancels the rest and prevents new calls.
- **Acceptance:** `TestResultsKeepInputOrder`, `TestNeverExceedsLimit`, `TestRunsConcurrently`, `TestFirstErrorCancelsTheRest`, `TestNoNewCallsAfterError`, `TestAlreadyCancelled`.
- <details><summary>Hint</summary>`errgroup.WithContext` + `SetLimit`; a goroutine can win its slot *after* another one failed, so check `ctx.Err()` at its start.</details>

### Exercise 3: First success
- **Task:** implement `FirstSuccess`: run all attempts, return the first success, cancel and wait for the others, join the errors if all fail.
- **Acceptance:** `TestFastSuccessWinsAndCancelsLosers`, `TestAllFailJoinsErrors`, `TestParentCancellation`, `TestNoFunctions`.
- <details><summary>Hint</summary>A results channel with capacity `len(fns)` lets late finishers exit; call `cancel()` and then `wg.Wait()` before returning.</details>

## 7. Recap
- Every goroutine needs an owner, a way to stop, and something that waits for it.
- Protect shared data with a mutex or atomics; share by channel when ownership moves; always test with `-race`.
- The sender closes a channel; `ctx.Done()` is how cancellation travels; cancelling is not waiting.
- Bound concurrency (`errgroup.SetLimit` or a fixed pool); results by index keep order for free.
- Start with a synchronous function and a mutex; reach for channels and pipelines when data flows between stages.

## 8. Self-check Questions
1. Why must `wg.Add` happen before `go`, and how does `wg.Go` remove that risk?
2. Who should close a channel with several senders, and how does `merge` in 3.2 decide when?
3. After `cancel()`, is the goroutine guaranteed to have stopped? What do you add to know?
4. Why is writing `results[i]` from different goroutines race-free but `results = append(results, v)` is not?
5. What happens to an `errgroup` with `SetLimit(2)` when you call `g.Go` a third time while two tasks run?

## 9. Further Reading
- [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Data Race Detector](https://go.dev/doc/articles/race_detector)
- [errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup) and [context](https://pkg.go.dev/context) package documentation

---
**Next:** [07 HTTP Client](../07-http-client/README.md)
