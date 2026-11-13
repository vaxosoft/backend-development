# P6: Job Worker Pool

**Modules exercised:** [06 Concurrency](../../modules/06-concurrency/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [13 Time and stdlib utilities](../../modules/13-time-and-stdlib-utilities/README.md).

## Goal

Run background work **inside** a FastAPI service: clients submit jobs over HTTP, a fixed pool of `asyncio` workers pulls them from a bounded queue, failed jobs are retried, slow jobs time out, jobs can be cancelled, and a graceful shutdown drains the queue before the process exits.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /jobs` | body `{"type": "reverse" \| "sleep" \| "fail", "payload": {...}}` -> `202` `{"id": "<uuid7>"}` and `Location: /jobs/{id}` |
| `GET /jobs/{id}` | `200` `{"id","type","status","attempts","result","error","created_at"}`; unknown id -> `404` `not_found` |
| `GET /jobs?limit=&offset=` | newest first: `{"items":[...],"total":N,"limit":L,"offset":O}`; `limit` default 20, values above 100 are clamped to 100; a non-numeric or `< 1` limit, or `offset < 0`, is `400` `bad_request` |
| `DELETE /jobs/{id}` | cancels a queued or running job -> `204`; finished job -> `409` `conflict`; unknown -> `404` |

Job types and payloads (validated before the job is queued; a bad payload is `422` `validation_failed` with `fields` such as `{"payload.ms": "..."}`; malformed JSON is `400` `bad_request`):

| Type | Payload | Result |
|------|---------|--------|
| `reverse` | `{"text": str}` | the reversed text (a string) |
| `sleep` | `{"ms": int}`, `0 <= ms <= 5000` (booleans are rejected) | `{"slept_ms": ms}`; the wait must be cancellable |
| `fail` | anything | never succeeds; every attempt raises `JobError("job failed on purpose")` |

Statuses: `queued` -> `running` -> `succeeded` \| `failed` \| `cancelled`. `created_at` is an RFC 3339 UTC timestamp; ids are UUID version 7, so they sort by creation time.

Pool rules (`Config`, all injectable; the tests use tiny values):

| Setting (env var) | Default | Meaning |
|-------------------|---------|---------|
| `workers` (`WORKERS`) | 4 | number of concurrently running jobs, never more |
| `queue_capacity` (`QUEUE_CAPACITY`) | 16 | bounded queue; a full queue answers `429` `rate_limited` with `Retry-After` and **stores nothing** |
| `max_attempts` (`MAX_ATTEMPTS`) | 3 | a failing or timed-out job is retried until it has been attempted this often, then `failed` (`attempts` counts them) |
| `retry_delay` (`RETRY_DELAY_MS`) | 200 ms | pause between attempts |
| `job_timeout` (`JOB_TIMEOUT_MS`) | 30 s | per attempt; on expiry the attempt fails with `timed out after ...` and is retried like any other failure |
| `drain_timeout` (`DRAIN_TIMEOUT_MS`) | 10 s | how long shutdown waits for queued and running jobs |

Cancelling a queued job leaves its slot in the queue until a worker dequeues and skips it. Cancelling a running job cancels its attempt (the worker is free again at once).

**Graceful shutdown** (lifespan exit): stop accepting (`POST /jobs` -> `503` `unavailable`), let running and queued jobs finish for at most `drain_timeout`, then cancel what is left (their status becomes `cancelled`) and stop the workers.

**Logging:** when a job reaches a final status the pool writes one `INFO` record on logger `p6_job_worker_pool` with `extra={"job_id", "status", "attempts"}`.

## Run

```bash
python -m pytest projects/P6-job-worker-pool/tests      # tests (from languages/python)
cd projects/P6-job-worker-pool/starter
ADDR=127.0.0.1:8080 WORKERS=2 python -m p6_job_worker_pool   # default ADDR is 127.0.0.1:8080
```

```bash
curl -i -X POST localhost:8080/jobs -H 'content-type: application/json' \
  -d '{"type":"reverse","payload":{"text":"hello"}}'          # 202 {"id":"..."}
curl localhost:8080/jobs/<id>                                  # "status":"succeeded","result":"olleh"
curl -X POST localhost:8080/jobs -H 'content-type: application/json' \
  -d '{"type":"sleep","payload":{"ms":3000}}'                  # then DELETE /jobs/<id> -> 204
curl -X POST localhost:8080/jobs -H 'content-type: application/json' \
  -d '{"type":"sleep","payload":{"ms":99999}}'                 # 422 with "payload.ms"
curl "localhost:8080/jobs?limit=5"
```

Ctrl-C and SIGTERM trigger the graceful shutdown: submit a `sleep` job, stop the server and watch it finish before the process exits.

## Acceptance criteria (tests)

The tests start the real app **with its lifespan** and talk to it through `httpx` over ASGI (`tests/p6_support.py`); no ports are used. pytest shows async test ids with an `[asyncio]` suffix.

| Criterion | Tests (`tests/test_p6_job_worker_pool.py`) |
|-----------|--------------------------------------------|
| submit and run | `test_post_job_returns_202_with_id_and_location`, `test_reverse_job_succeeds_with_reversed_text`, `test_sleep_job_succeeds_and_reports_slept_ms` |
| ids and timestamps | `test_job_ids_are_uuid7_and_created_at_is_utc` |
| validation | `test_unknown_job_type_is_422_with_envelope`, `test_invalid_payload_is_422_naming_the_field`, `test_malformed_json_is_400_bad_request` |
| reading | `test_unknown_job_is_404_with_envelope`, `test_list_returns_newest_first_with_paging_metadata`, `test_list_clamps_limit_and_applies_offset`, `test_list_with_non_numeric_limit_is_400` |
| retries, timeout | `test_failing_job_is_retried_until_max_attempts_then_failed`, `test_job_exceeding_the_timeout_fails_with_timeout_error` |
| pool and queue | `test_at_most_workers_jobs_run_at_once`, `test_full_queue_answers_429_with_retry_after`, `test_rejected_job_is_not_stored_and_pool_recovers` |
| cancellation | `test_cancel_queued_job_marks_it_cancelled_and_it_never_runs`, `test_cancel_running_job_frees_the_worker`, `test_cancel_finished_job_is_409_conflict`, `test_cancel_unknown_job_is_404` |
| graceful shutdown | `test_shutdown_lets_running_and_queued_jobs_finish`, `test_shutdown_stops_accepting_new_jobs_with_503`, `test_drain_timeout_cancels_jobs_that_do_not_finish_in_time` |
| logging | `test_finished_job_is_logged_with_job_id_status_and_attempts` |

## What to implement

The starter has the HTTP layer, error envelope, models, configuration and lifespan wiring. Every `NotImplementedError("TODO: ...")` is yours:

| File | Work |
|------|------|
| `starter/p6_job_worker_pool/handlers.py` | `validate_payload` and `run_job` (the three job types) |
| `starter/p6_job_worker_pool/pool.py` | `WorkerPool`: `start`, `submit`, `get`, `list_newest_first`, `cancel`, `shutdown`, plus the worker loop, the retry loop and logging |

Tests import `p6_job_worker_pool` from `starter/`; the reference solution overlays exactly those two files.

<details>
<summary>Hints</summary>

- `asyncio.Queue(maxsize=n).put_nowait` raises `asyncio.QueueFull`; translate it into the pool's own `QueueFull`. Create the `Job` first, but store it in `_jobs` only after the queue accepted it.
- A worker is `while True: job = await queue.get(); ...; queue.task_done()` (put `task_done` in a `finally`). `await queue.join()` then means "everything queued has been handled", and you can bound it with `async with asyncio.timeout(...)`.
- Run the attempts of one job in their **own task** (`asyncio.create_task`) and `await asyncio.wait({task})`. Cancelling that inner task cancels the job without killing the worker. Keep a `job id -> task` dict for `cancel`.
- `cancel` of a queued job only sets the status; the worker checks `job.status is JobStatus.QUEUED` after `get()` and skips the job otherwise.
- `asyncio.timeout(seconds)` raises `TimeoutError` at the end of the `async with` block. Catch `TimeoutError` and `JobError` per attempt and decide whether another attempt is left.
- Keep references to the worker tasks (a task nobody references can be garbage collected) and `await asyncio.gather(*workers, return_exceptions=True)` after cancelling them.
- In `shutdown`, set the closing flag first so `submit` raises `PoolClosed`; after the drain, mark jobs that are still `QUEUED` as cancelled.
- `logger.info("...", extra={"job_id": ...})` puts the fields on the log record; do not use f-strings in log calls.
- The route functions are `async def` on purpose: the pool is not thread-safe, a plain `def` route would run in a thread pool.

</details>

## Stretch goals (not tested)

- Exponential backoff with jitter instead of a constant `retry_delay`.
- A `GET /stats` endpoint with queue depth and per-status counts.
- Drop finished jobs older than a TTL so the in-memory store does not grow forever.
- Remove cancelled jobs from the queue immediately (for example by keeping the queue as a deque and a `Condition`).
- Run a CPU-bound job type in a `ProcessPoolExecutor` via `loop.run_in_executor` and observe that the loop stays responsive.

## Reference solution

[`solution/`](./solution/) holds only the files that differ from the starter (`handlers.py` and `pool.py`, same relative paths). Overlay it onto `starter/` to run the whole suite green.
