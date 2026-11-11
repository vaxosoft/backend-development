# P6 - Job Worker Pool

Build a background job system **inside** an HTTP service: clients submit jobs, a fixed pool of workers runs them from a bounded queue, and the service shuts down without losing work.

## Goal

An in-process worker pool (`Pool`) with retries, per-attempt timeouts, cancellation and graceful shutdown, exposed through a small JSON API.

## Requirements

All errors use the shared envelope `{"error":{"code","message","fields"?}}`. Ids are integers starting at 1.

| Endpoint | Behaviour |
|----------|-----------|
| `POST /jobs` | Body `{"type","payload"}`. `202` `{"id":N}` + `Location: /jobs/N`. Malformed JSON `400 bad_request`; body > 1 MiB `413 payload_too_large`; invalid job `422 validation_failed` with `fields`; queue full `429 rate_limited` + `Retry-After`; pool shut down `503 unavailable`. A rejected job is not stored. |
| `GET /jobs/{id}` | `200 {"id","type","status","attempts","result","error"}` (`result`/`error` are `null` when absent). Unknown or non-numeric id: `404 not_found`. |
| `GET /jobs` | `200 {"items":[...],"total","limit","offset"}`, newest first. `limit` default 20, clamped to 100; `offset` default 0; negative or non-numeric: `400 bad_request`. |
| `DELETE /jobs/{id}` | Cancels a queued or running job: `204`. Unknown id `404`; already finished (succeeded, failed or cancelled) `409 conflict`. |

Job types:

| `type` | `payload` | Result |
|--------|-----------|--------|
| `reverse` | `{"text": string}` (required) | the text reversed by rune |
| `sleep` | `{"ms": int}` (required, 0-5000), cancellable | `"slept 20ms"` |
| `fail` | ignored | always fails with `job failed on purpose` |

Invalid payloads report `fields` keys `type`, `payload.text` or `payload.ms`.

Pool rules:

- `Workers` goroutines (default 4) pull from a queue with `QueueSize` slots (default 64). Submitting never blocks.
- Statuses: `queued -> running -> succeeded | failed | cancelled`. A queued job that is cancelled never runs (`attempts` stays 0).
- `MaxAttempts` (default 3) is the **total** number of attempts. Between attempts the worker waits `RetryDelay` (tests use 1 ms); the status stays `running`.
- Each attempt gets its own `JobTimeout`. A timeout is a failed attempt and is retried like any other failure.
- Cancelling a running job stops it promptly and frees its worker.
- `Service.Shutdown(ctx)`: stop accepting (`503`), let queued and running jobs finish; if `ctx` ends first, cancel what is still running and return `ctx.Err()`.
- `Run(ctx, listener, cfg)`: serve until `ctx` is cancelled, then stop the HTTP server and drain the pool within `cfg.DrainTimeout`.

## Acceptance criteria (tests in `tests/jobs_test.go`)

| Criterion | Test |
|-----------|------|
| Create returns 202, id, Location | `TestCreateJobReturns202WithIDAndLocation` |
| `reverse` works on Unicode | `TestReverseJobSucceedsWithReversedText` |
| `sleep` succeeds | `TestSleepJobSucceeds` |
| Retries up to `MaxAttempts`, then `failed` | `TestFailJobRetriesUntilMaxAttemptsThenFails`, `TestMaxAttemptsIsConfigurable` |
| Per-attempt timeout | `TestJobTimeoutFailsTheAttempt` |
| Validation 422 with field names; limits | `TestInvalidJobsAreRejectedWith422AndFieldErrors`, `TestSleepAtTheLimitIsAccepted` |
| Malformed JSON 400, unknown id 404 | `TestMalformedJSONIs400`, `TestUnknownJobIs404` |
| Listing: order, envelope, paging, clamp, bad input | `TestListJobsIsNewestFirst`, `TestListJobsPaginatesAndClampsLimit`, `TestListJobsRejectsBadPagingWith400` |
| Bounded queue, 429 + `Retry-After`, nothing stored | `TestFullQueueReturns429WithRetryAfter` |
| Fixed number of workers | `TestOnlyConfiguredWorkersRunAtOnce` |
| Cancel queued / running / finished / unknown | `TestCancelQueuedJobNeverRuns`, `TestCancelRunningJobStopsIt`, `TestCancelledRunningJobFreesItsWorker`, `TestCancelFinishedOrUnknownJob` |
| Shutdown drains, rejects, force-cancels | `TestShutdownDrainsQueuedJobs`, `TestShutdownRejectsNewJobsWith503`, `TestShutdownDrainTimeoutCancelsRunningJobs` |
| `Run` drains running jobs, then refuses connections | `TestRunDrainsJobsThenStopsListening` |

## Running

From `languages/go/`:

```bash
ADDR=127.0.0.1:8080 WORKERS=4 QUEUE_SIZE=64 go run ./projects/P6-job-worker-pool/starter/cmd/server
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `127.0.0.1:8080` | listen address |
| `WORKERS` | `4` | worker goroutines |
| `QUEUE_SIZE` | `64` | queue capacity |

```bash
curl -i -X POST localhost:8080/jobs -d '{"type":"reverse","payload":{"text":"hello"}}'
curl localhost:8080/jobs/1
curl -X DELETE localhost:8080/jobs/1
```

Ctrl-C (or `SIGTERM`) triggers the graceful shutdown.

## Running the tests

```bash
go test ./projects/P6-job-worker-pool/tests/          # starter: fails until you implement it
go test -race ./projects/P6-job-worker-pool/tests/    # the race detector must stay quiet
```

The starter compiles and the HTTP handlers answer `501 not_implemented`, so tests fail on assertions.

## Suggested steps

1. `job.go`: implement `validate` and `execute`.
2. `pool.go`: `Submit`, `Get`, `List`, then the workers and `run` (status changes, attempts, retry loop).
3. `server.go`: the four handlers, then `Run`.
4. Add `Cancel` and `Shutdown` last, and run the tests with `-race`.

<details>
<summary>Hints</summary>

- Use one `sync.Mutex` for the job map **and** every job's fields; return copies (`JobView`) to callers.
- `select { case queue <- j: default: return ErrQueueFull }` is a non-blocking send. Do it while holding the lock so `Shutdown` cannot close the channel under you.
- Workers are `for j := range queue`; `Shutdown` closes the channel once, then waits on a `sync.WaitGroup`.
- Give every running job a context derived from a pool-wide context. `Cancel` calls the job's cancel func; forced shutdown cancels the pool context.
- Cancelling a queued job: just set its status to `cancelled`; the worker skips it when it dequeues it.
- Per attempt: `context.WithTimeout(jobCtx, cfg.JobTimeout)`. Tell a timeout (`context.DeadlineExceeded` from the attempt context only) from a cancel (the job context is done).
- Wait for retries with `time.NewTimer` + `select` on `ctx.Done()`, not `time.Sleep` or `time.After` in a loop.
- Once a job is `cancelled`, later updates from its worker must not overwrite that status.
- `Run`: `srv.Shutdown(drainCtx)` first (no new requests), then `svc.Shutdown(drainCtx)`.

</details>

## Modules exercised

- [06 Concurrency](../../modules/06-concurrency/README.md): workers, channels, `context`, `WaitGroup`, race detector
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): `slog` with per-job attributes
- [13 Time and stdlib utilities](../../modules/13-time-and-stdlib-utilities/README.md): `time.Duration`, timers, timeouts

## Stretch goals (not tested)

- Exponential backoff with jitter (`math/rand/v2`) between attempts.
- `GET /stats` with queue depth and counts per status via `expvar`.
- Evict finished jobs after a TTL so memory does not grow forever.
- Separate `high` and `low` priority queues.

## Reference solution

[`_solution/`](./_solution/) holds the complete `job.go`, `pool.go` and `server.go`; copy them over `starter/` to get a passing build. Try the exercise first.
