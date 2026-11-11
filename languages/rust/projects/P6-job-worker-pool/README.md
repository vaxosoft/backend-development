# P6: Job Worker Pool

Build a background worker pool that lives **inside** an HTTP service: clients submit jobs, a fixed set of worker tasks pulls them from a bounded queue, and the service shuts down without losing work.

Modules exercised: [06 Concurrency](../../modules/06-concurrency/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [13 Time and utilities](../../modules/13-time-and-stdlib-utilities/README.md) (timeouts, delays, `chrono` timestamps, UUIDs).

## Goal

`POST /jobs` answers immediately with `202`; the work happens later on one of `workers` Tokio tasks. Everything shared between handlers and workers (registry, queue, cancellation) is yours to get right: no blocking the runtime, no lock held across `.await`, no leaked tasks.

## What is given and what you write

| File | State | Content |
|------|-------|---------|
| `src/lib.rs`, `src/main.rs` | given | `Config` (+ env vars), `build_app`, `run`, signal handling |
| `src/http.rs` | given | routes, handlers, error envelope, `Retry-After`, panic -> 500 |
| `src/jobs.rs` | given | `JobSpec` + validation, `JobStatus`, `JobView` (JSON shape) |
| `src/exec.rs` | **TODO** | `execute`: what `reverse`, `sleep` and `fail` do |
| `src/pool.rs` | **TODO** | `submit`, `cancel`, `drain`, `run_job` (the worker loop and registry helpers are given) |

Search for `TODO` / `todo!()`. The doc comment above each stub lists its rules.

## Requirements

### Endpoints

| Request | Success | Errors |
|---------|---------|--------|
| `POST /jobs` `{"type":"reverse"\|"sleep"\|"fail","payload":{...}}` | `202` `{"id":"..."}` | `400 bad_request` malformed JSON; `422 validation_failed` (+ `fields`) unknown type, missing `payload.text`, `payload.ms` missing or over 5000; `429 rate_limited` + `Retry-After` queue full |
| `GET /jobs/{id}` | `200` `{"id","type","status","attempts","result","error","created_at"}` | `404 not_found` |
| `GET /jobs` | `200` `{"items":[...],"total":N}`, newest first | |
| `DELETE /jobs/{id}` | `204` (queued or running job) | `404 not_found`, `409 conflict` already finished |

Errors use the shared envelope `{"error":{"code","message","fields"?}}` (see [domain-and-projects](../../../../framework/domain-and-projects.md)).

### Job types

| `type` | payload | result |
|--------|---------|--------|
| `reverse` | `{"text": "..."}` | the text reversed by `char` |
| `sleep` | `{"ms": 0..=5000}` | `"slept {ms}ms"`; must be cancellable |
| `fail` | none | always an error (`"job always fails"`) |

### Pool behaviour

- `workers` tasks (default 4) pull job ids from a **bounded** queue of `queue_capacity` (default 64). Submitting never waits: when the queue is full the job is rejected (429) and **not stored**.
- Status flow: `queued` -> `running` -> `succeeded` | `failed`; `cancelled` from `queued` or `running`. `attempts` counts started attempts.
- A failed attempt (error **or** timeout) is retried until `max_attempts` (default 3), waiting `retry_delay` between attempts. The last error is kept in `error`; `result` stays `null`.
- Each attempt is bounded by `job_timeout`; a timeout produces an error containing `timed out`.
- `DELETE` marks the job `cancelled` immediately. A queued job never runs; a running attempt is dropped and **its worker is free again** at once.
- Graceful shutdown (`run`): stop accepting connections, let in-flight requests finish, then `Pool::drain`: close the queue, let queued and running jobs finish for at most `drain_timeout`, cancel whatever is left, join the workers. After `drain`, `submit` fails with `ShuttingDown`.

### Configuration

| Variable | Default | |
|----------|---------|---|
| `ADDR` | `127.0.0.1:8080` | listen address |
| `WORKERS` | `4` | worker tasks |
| `QUEUE_CAPACITY` | `64` | queued jobs |
| `MAX_ATTEMPTS` | `3` | attempts per job |
| `RETRY_DELAY_MS` | `200` | delay between attempts |
| `JOB_TIMEOUT_MS` | `10000` | per-attempt timeout |
| `DRAIN_TIMEOUT_MS` | `10000` | shutdown budget |
| `RUST_LOG` | `info` | tracing filter |

## Run

From `languages/rust/`:

```bash
ADDR=127.0.0.1:8080 cargo run -p p6-job-worker-pool

curl -i -X POST localhost:8080/jobs -d '{"type":"reverse","payload":{"text":"hello"}}'
curl localhost:8080/jobs
curl -i -X DELETE localhost:8080/jobs/<id>
```

Ctrl-C or `SIGTERM` starts the graceful shutdown.

## Tests

```bash
cargo test -p p6-job-worker-pool
```

The tests (`starter/tests/api.rs`) start a real server on port 0 and talk to it with `reqwest`; they use millisecond delays only. In the starter, most of them fail because `submit` panics (the router turns that into a 500).

## Acceptance criteria

| Criterion | Tests |
|-----------|-------|
| Submit answers 202 with an id; `build_app` works | `build_app_returns_a_router_that_accepts_jobs`, `post_job_answers_202_with_an_id` |
| `reverse` and `sleep` run | `reverse_job_succeeds_with_reversed_text`, `sleep_job_succeeds` |
| Validation and error envelope | `malformed_json_is_400_bad_request`, `unknown_job_type_is_422_validation_failed`, `reverse_without_text_is_422`, `sleep_over_5000_ms_is_422`, `unknown_job_is_404_with_error_envelope` |
| Listing, newest first | `list_returns_newest_job_first` |
| Fixed number of workers | `pool_runs_as_many_jobs_at_once_as_it_has_workers`, `extra_jobs_wait_in_the_queue_while_all_workers_are_busy` |
| Bounded queue, 429 + `Retry-After`, rejected job not stored | `full_queue_answers_429_with_retry_after` |
| Retries and `max_attempts` | `failing_job_is_retried_then_marked_failed`, `max_attempts_is_configurable` |
| Per-job timeout | `job_exceeding_the_timeout_fails_with_a_timeout_error` |
| Cancellation | `cancelling_a_running_job_answers_204_and_marks_it_cancelled`, `cancelling_a_running_job_frees_its_worker`, `cancelled_queued_job_never_runs`, `cancelling_a_finished_job_is_409`, `cancelling_an_unknown_job_is_404` |
| Graceful shutdown | `drain_lets_queued_jobs_finish`, `drain_closes_the_queue_for_new_jobs`, `drain_timeout_cancels_jobs_that_do_not_finish`, `run_finishes_running_jobs_then_stops_accepting_connections` |

## Suggested steps

1. `execute` (easy): `chars().rev()`, `tokio::time::sleep`, an `Err`.
2. `submit`, then `cancel`; run the tests that do not need a running job.
3. `run_job` without retries, then add the retry loop, the timeout, and cancellation.
4. `drain`.

<details>
<summary>Hints</summary>

- Insert the registry entry **before** `try_send`: a worker can dequeue the id immediately. On `TrySendError::Full` remove it again.
- Never hold a `std::sync::MutexGuard` across `.await`. The helpers (`begin`, `set_attempts`, `finish`) lock, change, and unlock in one synchronous step.
- One attempt: `tokio::select! { _ = token.cancelled() => return, res = tokio::time::timeout(job_timeout, execute(&spec)) => ... }`. Dropping the `execute` future *is* the cancellation. The retry delay needs the same `select!`.
- `cancel` already set the status, and `finish` ignores jobs that are finished, so after `token.cancelled()` simply return.
- `drain`: `queue.take()` drops the `Sender`, so workers see `None` after the last queued id. Use `std::mem::take` on the `JoinSet` to get it out of the mutex before awaiting. `join_next` is cancel-safe, so you can wrap the join loop in `tokio::time::timeout` and join again after cancelling the rest.
- `max_attempts` of 0 must still run once (`.max(1)`).
</details>

## Stretch goals (not tested)

- Remove finished jobs after a TTL with a `tokio::time::interval` sweeper (the registry grows forever now).
- `GET /jobs?status=failed&limit=10`.
- Per-job `finished_at` and a `/metrics` counter per status.
- Exponential backoff with jitter instead of a fixed `retry_delay`.

## Reference solution

`solution/src/exec.rs` and `solution/src/pool.rs` replace the starter files of the same name. Try it yourself first, then compare.
