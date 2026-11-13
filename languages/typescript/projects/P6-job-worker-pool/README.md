# P6 - Background Job Worker Pool

Build a small service that accepts jobs over HTTP and runs them on a **fixed pool of workers** inside the same process: a bounded queue, retries, per-job timeouts, cancellation and a graceful shutdown.

## Goal

- Model a worker pool with plain `async` functions, promises and `AbortController` (no threads, no queue library).
- Keep an HTTP API honest about back-pressure (`429` when the queue is full) and about its own lifecycle (drain on shutdown).
- Log one structured line per processed job.

## Modules exercised

- [06 Concurrency](../../modules/06-concurrency/README.md): promises, `AbortSignal`, bounded concurrency
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): pino, structured fields
- [13 Time and stdlib utilities](../../modules/13-time-and-stdlib-utilities/README.md): `timers/promises`, `AbortSignal.timeout`, `performance.now()`

## Requirements

Jobs are `{ "type", "payload" }` objects:

| `type`    | `payload`                              | Result                                                  |
| --------- | -------------------------------------- | ------------------------------------------------------- |
| `reverse` | `{ "text": string }` (max 10000 chars) | the text reversed by code points                        |
| `sleep`   | `{ "ms": integer 0-5000 }`             | `slept <ms>ms`; stops early when cancelled or timed out |
| `fail`    | optional object                        | always fails with `job failed on purpose`               |

Endpoints (errors use the shared envelope from [domain-and-projects.md](../../../../framework/domain-and-projects.md)):

| Request             | Success                                                  | Errors                                                                                                                                                                            |
| ------------------- | -------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /jobs`        | `202 {"id": 1}`                                          | `400 bad_request` (malformed JSON), `422 validation_failed` with `fields` (e.g. `payload.ms`), `429 rate_limited` + `Retry-After` (queue full), `503 unavailable` (shutting down) |
| `GET /jobs/{id}`    | `200 {"id","type","status","attempts","result","error"}` | `404 not_found`                                                                                                                                                                   |
| `GET /jobs`         | `200 {"items":[...],"total":N}`, newest first            |                                                                                                                                                                                   |
| `DELETE /jobs/{id}` | `204` (queued or running job becomes `cancelled`)        | `404 not_found`, `409 conflict` (already finished)                                                                                                                                |

`status` is one of `queued`, `running`, `succeeded`, `failed`, `cancelled`. `result` and `error` are `null` until set.

Pool behaviour (all values come from `Config`, see [`starter/src/config.ts`](./starter/src/config.ts)):

| Setting          | Env var            | Default | Meaning                                                                       |
| ---------------- | ------------------ | ------- | ----------------------------------------------------------------------------- |
| `workers`        | `WORKERS`          | 4       | jobs running at the same time                                                 |
| `queueCapacity`  | `QUEUE_CAPACITY`   | 100     | jobs waiting for a worker; running jobs do not count                          |
| `maxAttempts`    | `MAX_ATTEMPTS`     | 3       | total attempts per job (first run included)                                   |
| `retryDelayMs`   | `RETRY_DELAY_MS`   | 200     | pause between attempts                                                        |
| `jobTimeoutMs`   | `JOB_TIMEOUT_MS`   | 10000   | limit per attempt; a timeout is a failed attempt with `timed out after <n>ms` |
| `drainTimeoutMs` | `DRAIN_TIMEOUT_MS` | 10000   | how long shutdown waits for unfinished jobs before cancelling them            |

- Shutdown stops accepting jobs, lets running and queued jobs finish (up to the drain timeout), cancels the rest and only then resolves.
- Each processed job logs one `job finished` line with `jobId`, `type`, `status`, `attempts` and `durationMs`.

## What is already done

The skeleton compiles and runs; validation ([`schema.ts`](./starter/src/schema.ts)), error handling, `POST`/`GET` routes and `main.ts` are finished. **Your work is marked `TODO`:**

| File                                               | Task                                                                      |
| -------------------------------------------------- | ------------------------------------------------------------------------- |
| [`starter/src/runner.ts`](./starter/src/runner.ts) | `runJob`: the three job types, abort-aware                                |
| [`starter/src/pool.ts`](./starter/src/pool.ts)     | `WorkerPool`: queue, workers, retries, timeout, cancel, shutdown, logging |
| [`starter/src/app.ts`](./starter/src/app.ts)       | `DELETE /jobs/:id` and the `onClose` hook that drains the pool            |

## Run

From `languages/typescript/`:

```bash
ADDR=127.0.0.1:8080 node projects/P6-job-worker-pool/starter/src/main.ts
```

```bash
curl -i -X POST localhost:8080/jobs -H 'content-type: application/json' \
  -d '{"type":"reverse","payload":{"text":"hello"}}'
curl localhost:8080/jobs/1
curl -X DELETE localhost:8080/jobs/1
```

Ctrl-C (or `SIGTERM`) starts the graceful shutdown.

## Test

```bash
node --test "projects/P6-job-worker-pool/tests/**/*.test.ts"
```

The starter fails the tests until you implement the `TODO`s. Tests use `app.inject()` plus one real listener on port 0; the retry delay is 1 ms and the drain timeout 20 ms.

## Acceptance criteria

| Criterion                                | Test                                                                                                                                                                                                                    |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Accepting a job returns `202` and an id  | `POST /jobs accepts a job with 202 and a numeric id`                                                                                                                                                                    |
| `reverse` result, also for emoji         | `a reverse job succeeds with the reversed text`, `reverse works on code points, not UTF-16 units`                                                                                                                       |
| `sleep` works                            | `a sleep job succeeds after waiting`                                                                                                                                                                                    |
| Retries up to `maxAttempts` with a delay | `a failing job is retried ...`, `retries wait for the configured delay`                                                                                                                                                 |
| Per-job timeout                          | `a job that exceeds the job timeout fails with a timeout error`                                                                                                                                                         |
| Unknown job / bad input                  | `GET /jobs/{id} answers 404 ...`, `an unknown job type is a 422 ...`, `sleep longer than 5000 ms ...`, `malformed JSON is a 400 ...`                                                                                    |
| Newest first                             | `GET /jobs lists the newest job first`                                                                                                                                                                                  |
| Bounded queue                            | `a full queue answers 429 rate_limited with Retry-After`                                                                                                                                                                |
| Fixed number of workers                  | `at most workers jobs run at the same time`                                                                                                                                                                             |
| Cancellation                             | `DELETE cancels a queued job ...`, `DELETE cancels a running job and frees its worker`, `DELETE on a finished job is 409 conflict`, `DELETE on an unknown job is 404`                                                   |
| Graceful shutdown                        | `closing the app lets running and queued jobs finish first`, `jobs still unfinished after the drain timeout are cancelled`, `a pool that is shutting down rejects new jobs with 503`, `over a real socket, close() ...` |
| Logging                                  | `each processed job logs one structured "job finished" line`                                                                                                                                                            |

## Suggested steps

1. `runJob` first: `reverse` and `fail` are one-liners, `sleep` needs `timers/promises` with `{ signal }`. The pool-level tests need both parts, so try `runJob` from a scratch script first.
2. In `WorkerPool`, make `submit`, `get` and `list` work with a `Map` and an array queue; start the worker loops in the constructor.
3. Add retries and the timeout, then `cancel`, then `shutdown`.
4. Wire `DELETE` and the `onClose` hook in `app.ts`; add the log line.

<details>
<summary>Hint: idle workers without polling</summary>

An idle worker awaits a promise whose `resolve` function is stored in an `idle` array. `submit` calls `idle.shift()` and resolves it with the new entry, so no timer or polling is needed, and "queue full" only counts entries that really wait. Resolving with `undefined` tells the worker to exit.

</details>

<details>
<summary>Hint: cancel vs timeout</summary>

Give every entry an `AbortController`. For one attempt use `AbortSignal.any([controller.signal, AbortSignal.timeout(ms)])`. After a rejection, `controller.signal.aborted` means "cancelled by the user" and the timeout signal's `aborted` means "timed out". Set `status = 'cancelled'` inside `cancel()` itself so `GET` shows it immediately.

</details>

<details>
<summary>Hint: drain timeout</summary>

Race `Promise.all(workers)` against `sleep(drainTimeoutMs, true, { signal })` from `node:timers/promises`; abort the sleep when the workers win so no timer keeps the process alive. If the timer wins, cancel every unfinished job and await the workers again.

</details>

## Stretch goals (not tested)

- Evict finished jobs after some time (the map grows forever).
- Add `limit`/`offset` to `GET /jobs` following the list envelope of the REST conventions.
- Back-off that doubles `retryDelayMs` on every attempt.
- A `GET /stats` endpoint with queue length, running count and totals.

## Reference solution

The files that differ are in [`solution/`](./solution/) (same relative paths as in `starter/`). Copy them over `starter/` to see all tests pass.
