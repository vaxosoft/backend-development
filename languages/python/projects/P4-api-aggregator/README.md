# P4: API Aggregator

**Modules exercised:** [06 Concurrency](../../modules/06-concurrency/README.md), [07 HTTP client](../../modules/07-http-client/README.md).

## Goal

Build a small FastAPI service whose single endpoint fans out to an upstream API: `GET /aggregate?ids=1,2,3` fetches `GET {upstream}/items/{id}` for every id **concurrently**, but with a bound on how many calls are in flight, a per-call timeout, retries with exponential backoff and an overall deadline. Partial failure is a normal outcome, not an error.

## Requirements

| Topic | Behaviour |
|-------|-----------|
| Endpoint | `GET /aggregate?ids=1,2,3` calls `GET {upstream_url}/items/{id}` for each id |
| Validation | `ids` missing, empty, containing an empty entry (`1,,3`), non-numeric (`a`, `-1`) or with more than 50 entries: `400` and the envelope `{"error":{"code":"bad_request","message":"..."}}`. Exactly 50 is fine |
| Concurrency | all ids run concurrently, at most `max_concurrency` (default 4) upstream calls in flight |
| Timeout | each upstream call has `call_timeout` seconds (default 2.0) |
| Retries | up to `max_retries` (default 2) **retries** (so at most 3 calls) on network error, timeout and `5xx`; sleep `backoff_base * 2**(n-1)` before retry `n` (default base 0.1 s; tests use 1 ms). **Never retry 4xx** |
| Response | always `200` for a valid request: `{"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}`. Both lists keep **request order** |
| Error texts | `upstream status <code>`, `upstream timeout`, `upstream unreachable`, `deadline exceeded` |
| Deadline | after `overall_deadline` seconds (default 10) outstanding calls are cancelled and reported as `deadline exceeded`; finished ones keep their result |
| Disconnect | when the caller hangs up, the outstanding upstream calls are cancelled too (no retries are started either) |

Configuration (`Config`, read from the environment by `python -m p4_api_aggregator`):

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `127.0.0.1:8080` | listen address |
| `UPSTREAM_URL` | `http://127.0.0.1:9000` | base URL of the upstream |
| `MAX_CONCURRENCY`, `CALL_TIMEOUT`, `MAX_RETRIES`, `BACKOFF_BASE`, `OVERALL_DEADLINE` | see above | the knobs from the table |

## Run

```bash
pytest projects/P4-api-aggregator/tests      # tests (from languages/python)
cd projects/P4-api-aggregator/starter
UPSTREAM_ADDR=127.0.0.1:9000 python -m p4_api_aggregator.demo_upstream   # terminal 1: a toy upstream
ADDR=127.0.0.1:8080 python -m p4_api_aggregator                          # terminal 2: the aggregator
```

The demo upstream knows items 1-5, item 6 answers after 1 s, item 7 always fails with `500`, everything else is `404`.

```bash
curl "localhost:8080/aggregate?ids=3,1,2"
curl "localhost:8080/aggregate?ids=2,7,8,1"   # errors for 7 (500, retried) and 8 (404, not retried)
curl -i "localhost:8080/aggregate?ids=a,b"    # 400 bad_request
```

Ctrl-C and SIGTERM stop the server gracefully (uvicorn).

## Acceptance criteria (tests)

The tests start a **fake upstream** on a free loopback port (`tests/p4_fake_upstream.py`). It counts calls per id and concurrent calls, and can inject `5xx`, `4xx` and delays.

| Criterion | Tests (`tests/test_p4_aggregate.py`) |
|-----------|--------------------------------------|
| validation | `test_invalid_ids_are_rejected_with_400`, `test_exactly_fifty_ids_are_accepted` |
| response shape | `test_returns_upstream_json_for_each_id`, `test_partial_failure_is_still_200_with_errors_listed` |
| request order | `test_results_keep_request_order_even_when_upstream_answers_out_of_order` |
| bounded concurrency | `test_calls_run_concurrently_but_never_exceed_max_concurrency`, `test_max_concurrency_one_runs_calls_one_at_a_time` |
| retry policy | `test_5xx_is_retried_until_it_succeeds`, `test_gives_up_after_two_retries`, `test_4xx_is_never_retried`, `test_timeouts_are_retried_and_reported`, `test_unreachable_upstream_is_reported_per_id`, `test_backoff_doubles_between_retries` |
| deadline | `test_overall_deadline_cancels_slow_calls_and_keeps_finished_ones` |
| cancellation | `test_client_disconnect_cancels_outstanding_upstream_calls` |

## What to implement

The starter already has the app wiring (a pooled `httpx.AsyncClient` created in the lifespan, the `400` handler, rendering of the response). Every `NotImplementedError` is yours:

| File | Work |
|------|------|
| `starter/p4_api_aggregator/aggregator.py` | `parse_ids`, `fetch_item` (timeout, retry, backoff), `aggregate` (semaphore, `TaskGroup`, deadline, ordering) |
| `starter/p4_api_aggregator/app.py` | stop the work when the client disconnects (the starter simply awaits `aggregate`) |

Tests import `p4_api_aggregator` from `starter/`; the solution is overlaid on the same files.

<details>
<summary>Hints</summary>

- Catch `httpx.TimeoutException` **before** `httpx.TransportError`: timeouts are a subclass of transport errors. `httpx.HTTPStatusError` only exists if you call `raise_for_status()`; checking `response.status_code` is simpler here.
- Pass `timeout=` per request (`client.get(url, timeout=...)`). The client is shared; do not create one per call.
- `asyncio.Semaphore(n)` bounds the calls; take it around the **whole** `fetch_item` (retries included), otherwise retries can exceed the limit.
- Write each outcome into `outcomes[index]` from the worker, so a result that finished before the deadline survives the cancellation of the others.
- `async with asyncio.timeout(deadline), asyncio.TaskGroup() as group:` cancels every worker at the deadline and raises `TimeoutError` after the block; catch it and fill the gaps with `Failure(id, "deadline exceeded")`.
- Disconnect: run `aggregate` in a task next to a second task that polls `await request.is_disconnected()` every ~50 ms, `asyncio.wait(..., return_when=FIRST_COMPLETED)`, then cancel whichever is left. Cancelling a task cancels the `await client.get(...)` inside it and closes the connection.
- Catch `httpx` errors narrowly instead of `except Exception`, which hides bugs. `CancelledError` is a `BaseException`, so it passes through; never catch it without re-raising.

</details>

## Stretch goals (not tested)

- Cache successful items for a few seconds and share one in-flight call between concurrent requests for the same id.
- Add jitter to the backoff and honour an upstream `Retry-After` header.
- Add a circuit breaker: after N consecutive failures stop calling the upstream for a cool-down period.
- Stream the response as NDJSON while results arrive (module 07's streaming).

## Reference solution

[`solution/`](./solution/) holds only the two files that differ from the starter (`aggregator.py`, `app.py`). Try the project first, then compare.
