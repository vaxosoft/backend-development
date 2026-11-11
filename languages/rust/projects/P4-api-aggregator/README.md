# P4: API Aggregator

**Time:** 2-3 hours | **Difficulty:** Intermediate | **Modules:** [06 Concurrency](../../modules/06-concurrency/README.md), [07 HTTP client](../../modules/07-http-client/README.md)

## Goal
Build a small service whose single endpoint `GET /aggregate?ids=1,2,3` calls an upstream API for every id **concurrently**, retries flaky calls, and merges the answers in request order. One slow or broken upstream item must never break the whole response, and a client that goes away must stop costing upstream calls.

## Requirements
| Area | Rule |
|------|------|
| Endpoint | `GET /aggregate?ids=1,2,3` calls `GET {upstream_base}/items/{id}` for each id |
| Validation | `ids` missing, empty, a blank/non-numeric/negative entry, or more than 50 entries -> `400` with `{"error":{"code":"bad_request","message":"..."}}`. Spaces around entries are allowed. Nothing is sent upstream for a bad request |
| Concurrency | all ids run concurrently, never more than `max_concurrency` (default 4) in flight |
| Per-call timeout | every attempt is limited to `call_timeout` |
| Retries | up to `max_retries` (default 2) extra attempts on network error, timeout or 5xx; backoff `backoff_base`, `2x`, `4x`...; **never** retry 4xx |
| Response | `200` even with partial failures: `{"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}`; both lists keep request order; `errors` is `[]` when all succeed |
| Error texts | `upstream status <code>`, `upstream timeout`, `upstream unreachable`, `invalid upstream response`, `deadline exceeded` |
| Deadline | `overall_timeout` covers the whole request. Ids unfinished by then fail with `deadline exceeded`; their upstream calls are cancelled |
| Disconnect | when the client disconnects, outstanding upstream calls are cancelled |

Configuration (see `src/main.rs`): `ADDR` (default `127.0.0.1:8080`), `UPSTREAM_BASE`, `MAX_CONCURRENCY`, `MAX_RETRIES`, `CALL_TIMEOUT_MS`, `BACKOFF_BASE_MS`, `OVERALL_TIMEOUT_MS`.

## Your work
The starter compiles and serves, but three functions are `todo!()`:

| File | Function | Job |
|------|----------|-----|
| `starter/src/ids.rs` | `parse_ids` | validate and parse the query value |
| `starter/src/upstream.rs` | `fetch_with_retry` | one id: timeout, retry, backoff, error classification |
| `starter/src/aggregate.rs` | `run` | fan-out with bounded concurrency, order, overall deadline |

`Config`, `AppState`, the router, the response types and `main` are provided.

## Run the tests
From `languages/rust/`:
```bash
export CARGO_TARGET_DIR=/tmp/target   # optional, faster on a Windows-mounted drive
cargo test -p p4-api-aggregator
```
The tests start a **fake upstream** (an axum server on port 0 that counts attempts, tracks how many calls are in flight at once, and injects delays and failures) and the aggregator on another random loopback port. They run in well under a second once solved.

## Acceptance criteria -> tests
| Criterion | Test |
|-----------|------|
| 400 + envelope for missing / empty / malformed ids | `missing_ids_is_400_with_error_envelope`, `empty_ids_is_400`, `non_numeric_negative_or_blank_entries_are_400` |
| 50 ids allowed, 51 rejected; spaces tolerated | `more_than_50_ids_is_400_but_exactly_50_is_accepted`, `spaces_around_ids_are_tolerated` |
| Upstream JSON passed through | `upstream_json_is_passed_through_as_data` |
| Request order kept | `results_keep_request_order_even_when_upstream_answers_out_of_order` |
| Concurrent, bounded | `upstream_calls_run_concurrently`, `in_flight_upstream_calls_never_exceed_max_concurrency`, `max_concurrency_of_one_serialises_calls` |
| Partial failure is 200 | `partial_failure_is_200_with_results_and_errors` |
| Retry rules | `server_errors_are_retried_until_success`, `retries_stop_after_two_extra_attempts`, `max_retries_is_configurable`, `client_errors_are_never_retried` |
| Per-call timeout | `slow_upstream_hits_the_per_call_timeout_and_is_retried` |
| Network error | `unreachable_upstream_is_reported_per_id` |
| Overall deadline cancels | `overall_deadline_returns_partial_results_and_cancels_the_rest`, `ids_still_queued_at_the_deadline_fail_without_reaching_the_upstream` |
| Client disconnect cancels | `client_disconnect_cancels_outstanding_upstream_calls` |

## Try it by hand
Any server that answers `GET /items/{id}` with JSON works as the upstream (for example a second terminal running your own stub). Then:
```bash
UPSTREAM_BASE=http://127.0.0.1:9000 ADDR=127.0.0.1:8080 cargo run -p p4-api-aggregator
curl -s 'http://127.0.0.1:8080/aggregate?ids=1,2,3'
curl -s -w ' [%{http_code}]\n' 'http://127.0.0.1:8080/aggregate?ids=1,abc'
```
Expected shape (id 3 broken upstream):
```text
{"results":[{"id":1,"data":{"id":1,"name":"item-1"}},{"id":2,"data":{"id":2,"name":"item-2"}}],"errors":[{"id":3,"error":"upstream status 500"}]}
{"error":{"code":"bad_request","message":"`abc` is not a valid id"}} [400]
```

## Suggested steps
1. `parse_ids` first: it is pure, and 5 tests turn green.
2. `fetch_with_retry`: first a single attempt with a timeout, then the retry loop.
3. `run`: first a sequential loop (order and error handling), then make it concurrent, then the deadline.

<details>
<summary>Hint: one attempt</summary>

`client.get(url).timeout(config.call_timeout).send().await` returns an error with `is_timeout()`. Check `response.status()` yourself; `response.json::<Value>()` can fail too.
</details>

<details>
<summary>Hint: retry loop</summary>

Put the single attempt in its own function and loop: on `Err(e)` that is retryable and `retries < max_retries`, `tokio::time::sleep(base * 2u32.pow(retries))`, increment, continue; otherwise return. A `match` guard (`Err(e) if e.is_retryable() && ...`) keeps it short.
</details>

<details>
<summary>Hint: bounded concurrency in request order</summary>

`futures::stream::iter(ids).map(|id| async move { ... }).buffered(n).collect::<Vec<_>>().await` runs at most `n` futures at once and yields results in input order. `buffer_unordered` would break the order. A `tokio::sync::Semaphore` with `join_all` works too.
</details>

<details>
<summary>Hint: deadline and cancellation</summary>

Compute `deadline = Instant::now() + overall_timeout` once and wrap each id in `tokio::time::timeout_at(deadline, ...)`: an absolute deadline also expires ids that are still waiting for a free slot. In Rust, cancellation is just dropping a future: when `timeout_at` fires or hyper drops the handler future because the client left, the in-flight `reqwest` requests are dropped and their connections closed. No token needed.
</details>

## Stretch goals (not tested)
- Add jitter to the backoff and honour an upstream `Retry-After` header.
- Cache successful items for a few seconds and coalesce concurrent requests for the same id.
- Cap the total number of upstream calls across all concurrent `/aggregate` requests with a shared `Semaphore`.
- Emit a `tracing` span per id with the attempt count.

## Reference solution
[`solution/`](./solution/) contains only the three files that differ. To try it:
```bash
cp -r projects/P4-api-aggregator/solution/. projects/P4-api-aggregator/starter/
cargo test -p p4-api-aggregator
```
Copy your own `starter/` somewhere first if you want to keep your work: the overlay overwrites it.
