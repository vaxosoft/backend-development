# P6: Job Worker Pool

Build a background worker pool **inside** an ASP.NET Core service: clients submit jobs over HTTP, a fixed pool of workers processes them from a bounded queue, and shutdown drains the work in flight.

## Goal

`POST /jobs` answers immediately with `202`; the work happens later on `workers` long-lived loops that read from a `Channel<Job>`. You handle back-pressure (full queue), retries, timeouts, cancellation and graceful shutdown.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /jobs` | Body `{"type":"reverse"\|"sleep"\|"fail","payload":{...}}`. `202` + `Location: /jobs/{id}` + `{"id":N}`. Invalid body: `422` (`type` unknown, `reverse` needs `payload.text` string, `sleep` needs `payload.ms` integer 0-5000). Malformed JSON: `400`. Queue full: `429` + `Retry-After`. Shutting down: `503`. |
| `GET /jobs/{id}` | `200` `{"id","type","status","attempts","result","error","created_at"}`; unknown id `404`. |
| `GET /jobs` | `200` `{"items":[...],"total":N}`, newest first. |
| `DELETE /jobs/{id}` | Queued or running job: cancel it, `204`. Finished job (succeeded, failed or already cancelled): `409`. Unknown: `404`. |

Errors are `application/problem+json` (RFC 9457) with the same status codes as the other tracks plus a `code` member (`validation_failed`, `not_found`, `rate_limited`, `conflict`, `unavailable`).

Job types and results:

| type | payload | result |
|------|---------|--------|
| `reverse` | `{"text": "..."}` | the text reversed by user-perceived character (emoji and combining marks stay intact) |
| `sleep` | `{"ms": 0..5000}` | `{"slept_ms": N}`; must be cancellable |
| `fail` | any | never succeeds; every attempt fails |

Status lifecycle: `queued` -> `running` -> `succeeded` \| `failed`, or `cancelled` from `queued`/`running`. `attempts` counts started attempts.

**Pool rules**

- `Jobs:Workers` (default 4) loops read from a queue of `Jobs:QueueCapacity` (default 100). A submit never waits: a full queue is an immediate `429`.
- A failed attempt (exception or timeout) is retried until `Jobs:MaxAttempts` (default 3) with `Jobs:RetryDelayMs` (default 200) between attempts; then the job is `failed` with an `error` message. Each attempt has a timeout of `Jobs:JobTimeoutMs` (default 30000); a timeout is a failure, not a cancellation.
- **Graceful shutdown** (`SIGINT`/`SIGTERM`/Ctrl-C): stop accepting, let running and queued jobs finish for at most `Jobs:DrainTimeoutMs` (default 10000), then cancel whatever is left (`cancelled`) and exit.
- `created_at` comes from the injected `TimeProvider`.

Configuration is the standard layering: `appsettings.json` section `Jobs`, overridden by environment variables such as `Jobs__Workers=8`. `ADDR` (`host:port`, default `127.0.0.1:8080`) is the listen address.

## Acceptance criteria

Run from `languages/csharp/`. Test class and method names are in `tests/`.

| # | Criterion | Tests |
|---|-----------|-------|
| 1 | Submit answers `202` with id and `Location` | `SubmitTests.Submit_returns_202_with_id_and_location` |
| 2 | Bad type/payload is `422` problem details, malformed JSON `400` | `Invalid_job_is_422_problem_details`, `Malformed_json_is_400` |
| 3 | Status endpoint, unknown id `404`, newest-first list | `Unknown_job_is_404_problem_details`, `List_returns_newest_job_first` |
| 4 | `reverse`, `sleep`, `fail` behave as specified | `ExecutionTests.Reverse_job_succeeds_with_reversed_text`, `Reverse_keeps_emoji_and_combining_marks_intact`, `Sleep_job_succeeds` |
| 5 | Retries up to `MaxAttempts`, then `failed` | `Failing_job_is_retried_up_to_max_attempts_then_fails`, `Max_attempts_is_configurable` |
| 6 | Per-attempt timeout | `Job_that_exceeds_the_timeout_fails_with_a_timeout_error` |
| 7 | Bounded queue, `429` + `Retry-After` | `Full_queue_returns_429_with_retry_after` |
| 8 | At most `Workers` jobs run at once | `Pool_runs_exactly_as_many_jobs_in_parallel_as_it_has_workers` |
| 9 | Cancel queued/running, `409` when finished, `404` unknown | `CancellationTests.*` (4 tests) |
| 10 | Shutdown drains, then cancels after the timeout, queue closes | `ShutdownTests.*` (3 tests) |
| 11 | Timestamps from `TimeProvider` | `Job_creation_time_comes_from_the_injected_clock` |

## Run

```bash
export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
cd languages/csharp
ADDR=127.0.0.1:8080 dotnet run --project projects/P6-job-worker-pool/starter
```

```bash
curl -si -X POST localhost:8080/jobs -H 'content-type: application/json' \
  -d '{"type":"reverse","payload":{"text":"hello"}}'
# HTTP/1.1 202 Accepted ... Location: /jobs/1 ... {"id":1}
curl -s localhost:8080/jobs/1
# {"id":1,"type":"reverse","status":"succeeded","attempts":1,"result":"olleh","error":null,"created_at":"..."}
curl -si -X POST localhost:8080/jobs -H 'content-type: application/json' -d '{"type":"sleep","payload":{"ms":9000}}'
# HTTP/1.1 422 ... "errors":{"payload.ms":["must be an integer between 0 and 5000"]},"code":"validation_failed"
```

Watch shutdown: submit a few `sleep` jobs, press Ctrl-C and see the pool finish them before the process exits.

## Tests

```bash
dotnet test projects/P6-job-worker-pool/tests
```

The tests start the app in-process with `WebApplicationFactory<Program>`, shrink the retry delay to 1 ms and the drain timeout to 200 ms, and poll job status over HTTP instead of sleeping. On the starter, about half of them fail on assertions (and some wait up to 3 s for a status that never comes). The starter already contains the wiring, the `Job` state machine, validation and the endpoints; your work is marked with `TODO`:

| File | TODO |
|------|------|
| `JobHandlers.cs` | `ExecuteAsync`: the three job types |
| `JobService.cs` | enqueue without waiting (`TryWrite`), cancel |
| `WorkerPool.cs` | `RunAsync` (attempts, retries, timeout, cancellation) and the bounded drain in `StopAsync` |

## Steps and hints

1. Make `reverse` and `sleep` work and `fail` throw `JobFailedException`, then enqueue jobs in `JobService.TrySubmit`.
2. Implement `RunAsync` for the happy path, then add retries, then the timeout, then cancellation.
3. Implement `Cancel`, then the drain in `StopAsync`.

<details>
<summary>Hint: reversing text</summary>

`string.Reverse()` / `Array.Reverse` on `char[]` splits surrogate pairs, so emoji become garbage. `System.Globalization.StringInfo.GetTextElementEnumerator` yields whole text elements (a base character plus its combining marks).
</details>

<details>
<summary>Hint: telling "timed out" from "cancelled"</summary>

Create a `CancellationTokenSource(timeout, timeProvider)` for the attempt and link it with `job.Token` and the shutdown token. When an `OperationCanceledException` arrives, check whether the *outer* token (job or shutdown) is cancelled: yes means cancel the job, no means the timeout fired and the attempt failed.
</details>

<details>
<summary>Hint: the drain</summary>

Complete the channel writer (`jobs.StopAccepting()`), then `await workers.WaitAsync(drainTimeout, time, ct)`. `WaitAsync` throws `TimeoutException` when the time is up; catch it and call `base.StopAsync`, which cancels the token that `ExecuteAsync` received. Workers use that token to cancel the job they run and to cancel (not run) the rest of the queue.
</details>

<details>
<summary>Hint: why not cancel the stopping token right away?</summary>

`BackgroundService.StopAsync` cancels `stoppingToken` first. If workers watched it directly, shutdown would abort every job instead of draining. That is why the pool delays `base.StopAsync`.
</details>

## Modules exercised

- [06 Concurrency](../../modules/06-concurrency/README.md): `Channel<T>`, worker loops, `CancellationToken`, linked sources, `lock`
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): structured `ILogger` messages from workers
- [13 Time and stdlib utilities](../../modules/13-time-and-stdlib-utilities/README.md): `TimeProvider`, `BackgroundService`, shutdown behaviour

## Stretch goals (not tested)

- Expose a `/metrics`-style endpoint: queue depth, running count, totals per status.
- Exponential backoff with jitter between attempts.
- Evict finished jobs after a retention period (the store grows forever now) using `PeriodicTimer`.
- Free the queue slot of a cancelled queued job immediately instead of when a worker pops it.
- `GET /jobs?status=running&limit=20` with paging.

## Reference solution

`solution/` contains only the three files that differ from `starter/`. Try the tests first; to compare, run the solved tests with a copy:

```bash
cp -r projects/P6-job-worker-pool/solution/. projects/P6-job-worker-pool/starter/   # overwrites your work
```
