# 06. Concurrency in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md), [02 HTTP Server](../02-http-server/README.md)

All commands run from `languages/csharp/`. Setup once per shell: `export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1`.

## Learning Outcomes
After this module you can:
- [ ] Write `async`/`await` code, start work concurrently with `Task.WhenAll`, and explain where exceptions surface.
- [ ] Protect shared state with `lock`, `Interlocked`, `ConcurrentDictionary` or `SemaphoreSlim`, and prefer message passing with `Channel<T>`.
- [ ] Build a bounded worker pool with fan-out/fan-in and back-pressure.
- [ ] Propagate cancellation and timeouts with `CancellationToken`, linked tokens and `WaitAsync`.
- [ ] Recognise race conditions, lost updates and async deadlocks.

## 1. Concept Overview
Concurrency means making progress on several things at once. *I/O-bound* work (HTTP calls, database queries) spends its time waiting, so one thread can juggle many operations; *CPU-bound* work needs more cores. Shared mutable state is where bugs live: two threads doing `count++` can lose an update. Two answers: *protect* the state (locks, atomics) or *don't share it* (pass messages through a queue). Long-running work must be *cancellable* and have a *deadline*, otherwise a slow dependency or a disconnected client ties up resources forever.

```
producer -> [bounded queue] -> worker 1 \
                           -> worker 2  -> [results] -> collector     (fan-out / fan-in)
                           -> worker 3 /
         back-pressure: a full queue makes the producer wait (or refuse, HTTP 429)
```

## 2. How C# Approaches It

| Need | Tool |
|------|------|
| Asynchronous operation | `Task` / `Task<T>` (`ValueTask` for hot paths that often complete synchronously) with `async`/`await` |
| Wait for many | `Task.WhenAll`, `Task.WhenAny`, `Parallel.ForEachAsync` (with `MaxDegreeOfParallelism`) |
| CPU-bound | `Task.Run`, `Parallel.For` |
| Shared state | `lock` (sync only), `Interlocked`, `ConcurrentDictionary`, `SemaphoreSlim` (async-friendly) |
| Message passing | `System.Threading.Channels.Channel<T>` (bounded/unbounded), `IAsyncEnumerable<T>` |
| Cancellation / time | `CancellationToken`, `CancellationTokenSource.CancelAfter`, `Task.WaitAsync(timeout)`, `TimeProvider` |

Idiomatic style: **async all the way** (`await` up the call chain, never `.Result`/`.Wait()`), `CancellationToken` as the last parameter of every async API, the `Async` suffix, and tasks started *then* awaited together. ASP.NET Core has no `SynchronizationContext`, so `ConfigureAwait(false)` is not needed in app code (libraries still use it). Avoid `async void` (except event handlers), `Task.Run` around already-async code, and unbounded `Task.WhenAll` over thousands of items. No third-party library is used: everything is in the BCL.

## 3. Guided Examples
Run: `dotnet run --project modules/06-concurrency/examples -- <name>`. These examples never listen on a port.

### 3.1 async/await basics
- **Goal:** start tasks concurrently, collect results, see where exceptions appear.
- **Run:** `dotnet run --project modules/06-concurrency/examples -- async-basics`
- **Code:** [examples/01_AsyncBasics.cs](examples/01_AsyncBasics.cs)
  ```csharp
  var slow = FetchTitleAsync(1, delayMs: 30);
  var fast = FetchTitleAsync(2, delayMs: 1);
  string[] titles = await Task.WhenAll(slow, fast, medium);   // results in argument order
  ```
- **Walkthrough:** calling an `async` method *starts* it; `await` only waits. `WhenAll` returns results in argument order regardless of finishing order. An exception inside an async method is stored in the task and rethrown at the `await`; with several failures `await` rethrows the first and the task's `Exception.InnerExceptions` has all of them. `Task.Run` is for CPU-bound work.
- **Expected output:**
  ```
  WhenAll keeps input order: task-1, task-2, task-3
  WhenAny winner: timeout
  created the failing task without throwing
  caught at await: boom
  await threw: one; all failures: 2
  primes below 5002: 669
  ```

### 3.2 Shared state
- **Goal:** see a lost update, then fix it four ways.
- **Run:** `dotnet run --project modules/06-concurrency/examples -- shared-state`
- **Code:** [examples/02_SharedState.cs](examples/02_SharedState.cs)
  ```csharp
  await RunWorkers(_ => racy++);                       // read-modify-write is three steps
  await RunWorkers(_ => Interlocked.Increment(ref atomic));
  await semaphore.WaitAsync(); try { ... await ... } finally { semaphore.Release(); }
  ```
- **Walkthrough:** eight tasks each add 100,000. `racy++` usually ends below 800,000 and the exact number changes per run, so the example prints only the part that is always true. `lock` and `Interlocked` are exact. `lock` cannot contain an `await` (compile error CS1996); `SemaphoreSlim(1, 1)` is the async critical section. `ConcurrentDictionary.AddOrUpdate` makes per-key counters thread-safe (the update delegate may run more than once, keep it pure).
- **Expected output:**
  ```
  plain int++        : never more than 800000 (True); usually fewer
  lock               : 800000
  Interlocked        : 800000
  ConcurrentDictionary: done=200000, in_progress=300000, todo=300000
  SemaphoreSlim(1,1) : 800
  ```

### 3.3 Channels, fan-out/fan-in, pipelines
- **Goal:** share work by passing messages instead of sharing state.
- **Run:** `dotnet run --project modules/06-concurrency/examples -- channels`
- **Code:** [examples/03_ChannelsPipeline.cs](examples/03_ChannelsPipeline.cs)
  ```csharp
  var numbers = Channel.CreateBounded<int>(new BoundedChannelOptions(4) { SingleWriter = true });
  await foreach (var n in numbers.Reader.ReadAllAsync()) { ... }
  numbers.Writer.Complete();             // tells readers "no more items"
  ```
- **Walkthrough:** three workers read one channel (fan-out) and write to a results channel (fan-in). The producer completes the writer; workers finish when it drains; only after *all* workers are done is the results writer completed, otherwise the collector waits forever. A bounded channel gives back-pressure: `WriteAsync` waits and `TryWrite` returns `false` when full. Async iterators (`IAsyncEnumerable<T>`) chain stages lazily.
- **Expected output:**
  ```
  squares (sorted): 1, 4, 9, 16, 25, 36, 49, 64, 81, 100
  sum: 385
  TryWrite into capacity 2: True, True, False
  pipeline: WRITE, DOCS, SHIP
  ```

### 3.4 Cancellation, timeouts and bounded parallelism
- **Goal:** stop work on a deadline or a caller's cancel; cap in-flight work.
- **Run:** `dotnet run --project modules/06-concurrency/examples -- cancellation`
- **Code:** [examples/04_CancellationAndTimeouts.cs](examples/04_CancellationAndTimeouts.cs)
  ```csharp
  using var linked = CancellationTokenSource.CreateLinkedTokenSource(caller.Token);
  linked.CancelAfter(TimeSpan.FromSeconds(30));
  await never.Task.WaitAsync(TimeSpan.FromMilliseconds(20));      // TimeoutException
  await Parallel.ForEachAsync(items, new ParallelOptions { MaxDegreeOfParallelism = 3 }, async (item, ct) => { ... });
  ```
- **Walkthrough:** cancellation is cooperative: you pass the token down and APIs throw `OperationCanceledException` when it fires. A linked token cancels on either source (client disconnect *or* our deadline; in ASP.NET Core `HttpContext.RequestAborted` is the caller token). `WaitAsync(timeout)` only stops *waiting*, the work keeps running unless it also observes a token. Both `Parallel.ForEachAsync` and `SemaphoreSlim` cap concurrency; the example records the peak in-flight count with `Interlocked`.
- **Expected output:**
  ```
  deadline: Task.Delay canceled
  linked: worker stopped after cancel, canceled=True
  WaitAsync: TimeoutException (the underlying task is still pending)
  Parallel.ForEachAsync: processed=20, max in flight <= 3: True
  SemaphoreSlim(2): peak <= 2: True
  ```

### 3.5 Task Tracker worker pool
- **Goal:** the shape of the background-job service from project P6: bounded queue, fixed workers, graceful stop.
- **Run:** `dotnet run --project modules/06-concurrency/examples -- worker-pool`
- **Code:** [examples/05_TrackerWorkerPool.cs](examples/05_TrackerWorkerPool.cs)
  ```csharp
  public bool TrySubmit(Job job) => _queue.Writer.TryWrite(job);    // false => HTTP 429
  public async Task StopAsync() { _queue.Writer.Complete(); await Task.WhenAll(_running); }
  ```
- **Walkthrough:** before the workers start only two jobs fit, so three are refused (a handler would answer `429`). After `Start()` the refused jobs are submitted with `SubmitAsync`, which waits for room. A failing job is caught inside the worker so the pool keeps running. `StopAsync` completes the writer and waits: queued jobs still run (graceful drain), and later submits are refused.
- **Expected output:**
  ```
  submit 1: accepted
  submit 2: accepted
  submit 3: queue full
  submit 4: queue full
  submit 5: queue full
  job 1: succeeded rekcart
  job 2: succeeded WRITE DOCS
  job 3: succeeded desserts
  job 4: failed unknown job type 'explode'
  job 5: succeeded DONE
  submit after stop: refused
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `.Result` / `.Wait()` / `GetAwaiter().GetResult()` | feels simpler than making callers async; blocks a thread-pool thread (deadlocks under a `SynchronizationContext`, starves the pool in a server) | `await` all the way up |
| `async void` method | exceptions cannot be observed and crash the process | return `Task`; `async void` only for event handlers |
| Fire-and-forget `_ = DoAsync();` | the exception is never observed, work is lost on shutdown | await it, or hand it to a queue/`BackgroundService` that logs failures |
| `lock (x) { await ... }` | compile error CS1996: a lock is thread-affine | `SemaphoreSlim.WaitAsync` or restructure to lock only around the sync part |
| `counter++` or `list.Add` from many tasks | read-modify-write is not atomic; `List<T>` is not thread-safe | `Interlocked`, `lock`, concurrent collection or a channel |
| `Task.Run(() => asyncCall())` in a request handler | extra thread hop with no gain; hides the real awaitable | just `await asyncCall()`; `Task.Run` is for CPU-bound work |
| Forgetting `Complete()` on a `Channel` writer | `await foreach` over the reader never ends | complete the writer (once all producers are done) |
| `Task.WhenAll` over 10,000 HTTP calls | everything starts at once | `Parallel.ForEachAsync` with `MaxDegreeOfParallelism`, or a `SemaphoreSlim` |
| Passing no token, or swallowing `OperationCanceledException` as an error | cancellation looks like a failure (500) or never happens | pass `ct` to every awaitable; treat `OperationCanceledException` from your own token as normal completion |
| `await Task.WhenAll(tasks)` shows one exception | `await` unwraps the first one | inspect `whenAll.Exception.InnerExceptions` when you need all |
| Sharing a `Random`, `HttpClient` state or `DbContext` across tasks incorrectly | assuming every type is thread-safe | check the docs; `DbContext` is per unit of work, `HttpClient` is safe to share |

## 5. Coming From Other Languages
- **Go:** a `Task` is not a goroutine: it is a value you can await, compose and cancel, scheduled on a thread pool, and a failed task keeps its exception until awaited. `Channel<T>` ≈ Go channels (but `Complete()` instead of `close`); `CancellationToken` ≈ `context.Context` (passed explicitly, last argument); `SemaphoreSlim` ≈ a buffered-channel semaphore; `-race` has no direct equivalent, so tests rely on stress loops and design.
- **Java:** `Task<T>` ≈ `CompletableFuture<T>` but with language-level `await`; `lock` ≈ `synchronized`; `Interlocked` ≈ `AtomicInteger`; `Parallel.ForEachAsync` ≈ a bounded `ExecutorService`.
- **TypeScript/JS:** same `async`/`await` syntax, but `Task`s run on real threads, so shared state *can* race, and a `Task` starts hot like a `Promise`. `Promise.all` ≈ `Task.WhenAll`; `AbortController` ≈ `CancellationTokenSource`.
- **Python:** `asyncio.gather` ≈ `WhenAll`; there is no GIL, so CPU-bound `Task.Run`/`Parallel.For` really use several cores.

## 6. Exercises
Starters and tests live in `exercises/` (project `M06.Exercises`); solutions in `solutions/`. Tests use task completion sources and bounded waits instead of sleeps.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Status counter | Easy | thread-safe state, atomic check-then-act | `dotnet test modules/06-concurrency/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Bounded parallel map | Medium | limiting concurrency, ordering, cancellation, error propagation | `dotnet test modules/06-concurrency/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Worker pool | Stretch | channels, back-pressure, graceful shutdown, forced cancel | `dotnet test modules/06-concurrency/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Status counter ([Ex1_StatusCounter.cs](exercises/Ex1_StatusCounter.cs))
- **Task:** make `Increment`, `Get`, `Total`, `Snapshot` and `TryIncrementBelow` safe under 8+ concurrent threads.
- **Acceptance:** `Ex1_StatusCounterTests`: 160,000 concurrent increments are all counted; a snapshot is a copy; with a limit of 100 exactly 100 of 2,000 racing callers succeed.
- **Hint:** a check followed by an increment is two steps: they must sit inside one `lock`.

### Exercise 2: Bounded parallel map ([Ex2_BoundedParallel.cs](exercises/Ex2_BoundedParallel.cs))
- **Task:** `MapAsync` runs `work` for each item, at most `maxConcurrency` at a time, returning results in input order.
- **Acceptance:** `Ex2_BoundedParallelTests`: order kept, peak concurrency is exactly the limit (and a sequential version times out), first failure stops new work and cancels in-flight items and rethrows the original exception, caller cancellation, invalid limit, empty input.
- **Hint:** write each result into `results[index]`; `Parallel.ForEachAsync` already cancels the token it hands out when a body throws.

### Exercise 3: Worker pool ([Ex3_WorkerPool.cs](exercises/Ex3_WorkerPool.cs))
- **Task:** a pool whose workers start in the constructor, with `TrySubmit`, counters and `ShutdownAsync(drainTimeout)`.
- **Acceptance:** `Ex3_WorkerPoolTests`: all jobs run and drain, at most `workers` run at once (and all do), full queue refuses, no submits after shutdown, a throwing job is counted without killing the worker, a non-draining shutdown cancels running jobs and returns `false`.
- **Hint:** a bounded `Channel` gives you `TryWrite`, `TryComplete` and `ReadAllAsync`; `Task.WhenAll(workers).WaitAsync(timeout)` throws `TimeoutException` when the drain takes too long.

## 7. Recap
- Await everything, pass a `CancellationToken` everywhere, never block on a task.
- Shared mutable state needs a lock/atomic; better, hand data over through a `Channel<T>`.
- Bound everything: queue capacity, in-flight work, and time (deadline tokens).
- Complete channel writers, and stop workers by draining, then cancelling if the drain deadline passes.
- I/O-bound: `async`/`await`. CPU-bound: `Task.Run`/`Parallel.For`. Shared counter: `Interlocked`. Many producers/consumers: `Channel<T>`.

## 8. Self-check Questions
1. Two requests call `await Task.Delay(1)` in a handler running on a single thread-pool thread: why does this not block the second request, and what changes if you call `.Wait()` instead?
2. Your `Channel<int>` consumer never finishes after all producers return. What did you forget, and in a fan-in with three workers *when* may the result writer be completed?
3. Why can `WaitAsync(TimeSpan.FromSeconds(1))` around an HTTP call still leave the call running, and how do you actually stop it?
4. `ConcurrentDictionary<string,int>` is thread-safe, yet `if (!d.ContainsKey(k)) d[k] = 1;` is a race. Which API fixes it?

## 9. Further Reading
- [Asynchronous programming with async and await](https://learn.microsoft.com/dotnet/csharp/asynchronous-programming/)
- [System.Threading.Channels](https://learn.microsoft.com/dotnet/core/extensions/channels)
- [Cancellation in managed threads](https://learn.microsoft.com/dotnet/standard/threading/cancellation-in-managed-threads)
- Stephen Toub, [How Async/Await Really Works in C#](https://devblogs.microsoft.com/dotnet/how-async-await-really-works/)

---
**Next:** [07 HTTP Client](../07-http-client/README.md)
