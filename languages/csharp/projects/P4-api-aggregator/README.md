# P4 - API Aggregator

Build a small gateway: `GET /aggregate?ids=1,2,3` asks an upstream service for each id **concurrently** (bounded), survives slow and flaky upstream calls, and answers with one combined document.

**Modules exercised:** [06 Concurrency](../../modules/06-concurrency/README.md), [07 HTTP client](../../modules/07-http-client/README.md).

## Requirements

| Topic | Rule |
|-------|------|
| Endpoint | `GET /aggregate?ids=1,2,3` |
| Upstream call | `GET {UpstreamBaseUrl}/items/{id}` for every id, JSON body |
| Concurrency | at most `MaxConcurrency` (default 4) upstream calls in flight, but ids are fetched in parallel up to that limit |
| Per-call timeout | each attempt is cancelled after `CallTimeoutMs` (default 2000) |
| Retries | up to `MaxRetries` (default 2) retries on network error, timeout and 5xx; delay before retry *n* (0-based) is `BackoffBaseMs * 2^n` (default 100 ms); **never** retry 4xx |
| Order | `results` and `errors` both follow the order of the requested ids |
| Success body | `{"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}` |
| Partial failure | still HTTP `200` |
| Error texts | `upstream status {code}`, `timeout`, `upstream unreachable`, `invalid upstream response`, `deadline exceeded` |
| Bad input | `400` `application/problem+json` when `ids` is missing/empty, has more than 50 entries, or has an entry that is not a non-negative integer (`1,a`, `1,,2`, `-1`, `1.5`); spaces around entries are ignored |
| Deadline | the whole request gets `OverallTimeoutMs` (default 10 000); outstanding calls are cancelled and their ids reported as `deadline exceeded` |
| Disconnect | when the client goes away, outstanding upstream calls are cancelled |

Errors use `application/problem+json` (RFC 9457) with the same status codes as the Go/Rust tracks' JSON envelope.

## Configuration

Section `Aggregator`, env vars use `__`:

| Env var | Default |
|---------|---------|
| `ADDR` | `127.0.0.1:8080` |
| `Aggregator__UpstreamBaseUrl` | `http://127.0.0.1:9000/` |
| `Aggregator__MaxConcurrency` | `4` |
| `Aggregator__CallTimeoutMs` | `2000` |
| `Aggregator__MaxRetries` | `2` |
| `Aggregator__BackoffBaseMs` | `100` |
| `Aggregator__OverallTimeoutMs` | `10000` |

## Run and test

All commands run from `languages/csharp/`.

```bash
ADDR=127.0.0.1:8080 Aggregator__UpstreamBaseUrl=http://127.0.0.1:9000 \
  dotnet run --project projects/P4-api-aggregator/starter
curl -i 'http://127.0.0.1:8080/aggregate?ids=1,2,3'

dotnet test projects/P4-api-aggregator/tests
```

The tests start a real Kestrel **fake upstream** on a loopback port (`tests/FakeUpstream.cs`) that counts calls and concurrent calls and can be scripted to fail, stall or drop connections, then call your app through `WebApplicationFactory<Program>`. They need no network and finish in a few seconds.

## What to implement

The starter compiles and wires everything (`Program.cs`, options, DTOs). Three `TODO(P4)` stubs are yours:

| File | Task |
|------|------|
| `starter/IdParser.cs` | parse and validate the `ids` query value |
| `starter/UpstreamClient.cs` | one item: per-attempt timeout, retry with backoff, error classification |
| `starter/Aggregator.cs` | bounded fan-out, ordering, partial failures, overall deadline, cancellation |

## Acceptance criteria -> tests (`tests/AggregateTests.cs`)

| Criterion | Test |
|-----------|------|
| invalid `ids` -> 400 problem+json | `Missing_ids_returns_400_problem_details`, `Empty_or_non_numeric_ids_return_400` |
| at most 50 ids | `More_than_50_ids_return_400_and_call_nothing`, `Exactly_50_ids_are_accepted` |
| whitespace tolerated | `Spaces_around_ids_are_ignored` |
| response shape | `Upstream_json_is_wrapped_in_data` |
| request order preserved | `Results_keep_request_order_even_when_upstream_answers_out_of_order` |
| partial failure is 200 | `Partial_failure_returns_200_with_results_and_errors_in_order` |
| concurrency bound | `Never_has_more_than_max_concurrency_calls_in_flight`, `Default_max_concurrency_is_4_and_calls_really_overlap` |
| retry on 5xx, then give up | `Retries_a_5xx_and_succeeds_on_the_third_attempt`, `Gives_up_after_two_retries_and_reports_the_status`, `Retries_follow_max_retries_setting` |
| never retry 4xx | `Does_not_retry_4xx` |
| network errors | `Retries_dropped_connections_then_recovers`, `Reports_unreachable_after_repeated_network_errors` |
| per-call timeout | `Per_call_timeout_is_retried_and_then_reported_as_timeout` |
| overall deadline cancels | `Overall_deadline_cancels_outstanding_upstream_calls` |
| client disconnect cancels | `Client_disconnect_cancels_outstanding_upstream_calls` |

## Suggested steps and hints

1. `IdParser`, then run the validation tests.
2. `UpstreamClient` with a single attempt, then add the retry loop.
3. `Aggregator`: first sequentially, then concurrently, then bound it.
4. Deadline and disconnect last.

<details>
<summary>Hint: validation</summary>

`raw.Split(',', StringSplitOptions.TrimEntries)` keeps empty entries (good: `1,,2` must fail). `int.TryParse(part, NumberStyles.None, ...)` rejects signs, dots and spaces.
</details>

<details>
<summary>Hint: per-attempt timeout</summary>

Create a fresh `CancellationTokenSource.CreateLinkedTokenSource(callerToken)` per attempt and call `CancelAfter`. If an `OperationCanceledException` arrives while the *caller's* token is **not** cancelled, it was the timeout: retry. If the caller's token is cancelled, let it propagate.
</details>

<details>
<summary>Hint: bounding concurrency</summary>

Start one task per id and make each wait on a `SemaphoreSlim(MaxConcurrency)` before calling the upstream (release in `finally`). `Task.WhenAll` returns results in input order. `Parallel.ForEachAsync` with `MaxDegreeOfParallelism` also works.
</details>

<details>
<summary>Hint: deadline vs disconnect</summary>

Link the request's token to a second source with `CancelAfter(OverallTimeoutMs)`. After `WhenAll`, check the *request* token: if it is cancelled the client left, so throw; otherwise any id that was cancelled is a `deadline exceeded` entry.
</details>

<details>
<summary>Hint: baseline HttpClient setup</summary>

`Program.cs` registers `UpstreamClient` as a typed client via `AddHttpClient<T>` (connection pooling handled by the factory) and disables `HttpClient.Timeout` so your own timeouts decide.
</details>

## Stretch goals (not tested)

- Replace the hand-written retry loop with `AddResilienceHandler` (`Microsoft.Extensions.Http.Resilience`) and compare.
- Add jitter to the backoff.
- Cache successful items for a few seconds with `IMemoryCache` and a `TimeProvider`.
- Expose a `/healthz` endpoint that checks the upstream.

## Reference solution

`solution/` holds only the three files that differ from `starter/`. Try the tasks first, then compare. To run the tests against the solution without touching the starter, copy `projects/P4-api-aggregator` elsewhere and overlay `solution/*` onto `starter/`.
