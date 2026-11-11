# P4 - API Aggregator (Go)

**Goal:** build a small service that answers one request by calling an upstream API many times **concurrently**, with a bounded number of calls in flight, per-call timeouts, retries with exponential backoff, and cancellation that reaches the upstream.

**Modules exercised:** [06 Concurrency](../../modules/06-concurrency/README.md), [07 HTTP client](../../modules/07-http-client/README.md) (also touches [03 REST API design](../../modules/03-rest-api-design/README.md) for the error envelope).

## Requirements

`GET /aggregate?ids=1,2,3` calls `GET {UPSTREAM_URL}/items/{id}` for every id.

| Rule | Detail |
|------|--------|
| Concurrency | all ids are fetched concurrently, at most `MaxConcurrency` (default 4) calls in flight |
| Per-call timeout | `CallTimeout` limits **one attempt** (default 2 s) |
| Retries | up to `MaxRetries` (default 2) retries on network error, timeout or 5xx; waits `BackoffBase`, then `2x`, `4x`... (tests use 1 ms); **never** retry 4xx or an invalid JSON body |
| Response | `200 {"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}`; both arrays keep request order and are `[]` (never `null`) when empty |
| Partial failure | still `200` |
| Bad input | `400` with the error envelope `{"error":{"code":"bad_request","message":"..."}}` when `ids` is missing, empty, has an empty or non-numeric/negative entry, or has more than 50 entries |
| Cancellation | the client disconnecting, or the overall deadline (`OverallTimeout`, default 10 s), cancels outstanding upstream calls; unfinished ids are reported as `"error":"cancelled"` |

Error messages in `errors[]`: `upstream status <code>`, `upstream timeout`, `upstream unreachable`, `upstream returned invalid JSON`, `cancelled`.

## Acceptance criteria -> tests

All tests are in [`tests/aggregator_test.go`](tests/aggregator_test.go) and drive the real handler over HTTP against a fake upstream that counts calls and concurrency.

| Criterion | Test |
|-----------|------|
| `ids` missing/empty -> 400 + envelope | `TestMissingIDsIs400WithErrorEnvelope`, `TestIDsParameterAbsentIs400` |
| non-numeric/negative/empty entries -> 400, no upstream call | `TestInvalidIDsAre400AndNeverReachUpstream` |
| more than 50 ids -> 400; 50 is fine | `TestMoreThan50IDsIs400`, `TestExactly50IDsAreAccepted` |
| request order, upstream JSON embedded as-is | `TestResultsKeepRequestOrderAndEmbedUpstreamJSON` |
| `errors` is `[]` not `null` | `TestEmptyErrorsEncodeAsArrayNotNull` |
| partial failure -> 200, ordered errors | `TestPartialFailureIs200WithErrorsInRequestOrder` |
| invalid upstream JSON, not retried | `TestInvalidUpstreamJSONIsReportedAndNotRetried` |
| 5xx retried twice, then reported | `TestServerErrorsAreRetriedTwiceThenReported` |
| retry can succeed | `TestTransientServerErrorRecovers` |
| 4xx never retried | `TestClientErrorsAreNeverRetried` |
| per-call timeout, then retry | `TestSlowCallTimesOutAndIsRetried` |
| network error retried | `TestDroppedConnectionIsRetried` |
| exponential backoff | `TestBackoffDoublesBetweenRetries` |
| concurrent fan-out | `TestCallsRunConcurrently` |
| bounded concurrency | `TestMaxConcurrencyBoundsCallsInFlight` |
| client disconnect cancels upstream | `TestClientDisconnectCancelsUpstreamCalls` |
| overall deadline cancels upstream | `TestOverallDeadlineCancelsUpstreamAndReportsEveryID` |

## What is given, what you write

[`starter/`](starter/) compiles and runs; the tests fail until you implement three functions (each has a `TODO` comment with the details):

| File | You implement |
|------|---------------|
| [`ids.go`](starter/ids.go) | `ParseIDs` - validate and parse the `ids` query value |
| [`upstream.go`](starter/upstream.go) | `fetch` - one id: per-call timeout, retries, backoff, error mapping |
| [`fanout.go`](starter/fanout.go) | `Aggregate` - bounded concurrent fan-out, ordered merge |

Already provided: `Config`/`DefaultConfig`/`ConfigFromEnv`, response types, the handler (`aggregator.go`), `cmd/server` (reads `ADDR`) and `cmd/fakeupstream` (a toy upstream for manual tries).

## Run it

From `languages/go/`:

```bash
# terminal 1: toy upstream (id 13 -> 500, id 404 -> 404, id 99 -> 3 s delay)
ADDR=127.0.0.1:9000 go run ./projects/P4-api-aggregator/starter/cmd/fakeupstream

# terminal 2: the aggregator (Ctrl-C shuts it down gracefully)
UPSTREAM_URL=http://127.0.0.1:9000 ADDR=127.0.0.1:8080 go run ./projects/P4-api-aggregator/starter/cmd/server

curl -s 'http://127.0.0.1:8080/aggregate?ids=1,2,13,404'
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `127.0.0.1:8080` | listen address |
| `UPSTREAM_URL` | (required) | upstream base URL |
| `MAX_CONCURRENCY` | `4` | calls in flight |
| `MAX_RETRIES` | `2` | retries after the first attempt |
| `CALL_TIMEOUT` / `OVERALL_TIMEOUT` / `BACKOFF_BASE` | `2s` / `10s` / `100ms` | Go durations |

## Run the tests

```bash
go test ./projects/P4-api-aggregator/tests/            # fails until you are done
go test -race -count=3 ./projects/P4-api-aggregator/tests/   # needs gcc; catches data races
```

## Suggested steps

1. `ParseIDs` (the tests for 400 turn green).
2. `fetch` without retries: build the request with `http.NewRequestWithContext`, check the status, return the body.
3. Add the per-attempt timeout, then the retry loop with backoff.
4. `Aggregate` sequentially first, then make it concurrent and bounded.
5. Run with `-race`.

<details>
<summary>Hints</summary>

- `context.WithTimeout(ctx, cfg.CallTimeout)` per **attempt**; derive it from the incoming `ctx` so cancellation still flows. Read the body *before* the attempt's `cancel` runs.
- After an error, check the **parent** `ctx.Err()` first: a cancelled request is not a retryable timeout. Only then test `errors.Is(err, context.DeadlineExceeded)` to detect the per-call timeout.
- Backoff before attempt `n` (n >= 1): `BackoffBase << (n-1)`. Wait with a `time.Timer` and `select` on `ctx.Done()`, never a bare `time.Sleep`.
- `errgroup.Group` + `g.SetLimit(n)` blocks `g.Go` while `n` goroutines run. Return `nil` from the goroutines (a failed id is data, not a reason to cancel the others), and let each goroutine write only to its own slice index so no mutex is needed.
- `defer resp.Body.Close()`; drain small bodies of non-2xx answers so the connection can be reused.
- `make([]Failure, 0)` encodes as `[]`; a nil slice encodes as `null`.

</details>

## Stretch goals (not tested)

- Honour an upstream `Retry-After` header.
- Add jitter to the backoff.
- Cache successful items for a few seconds and deduplicate identical in-flight ids (`singleflight`).
- Add a per-request `X-Request-ID` and pass it to the upstream.

## Reference solution

[`_solution/`](_solution/) contains the files that differ (`ids.go`, `upstream.go`, `fanout.go`). To try it, overlay it on the starter (from `languages/go/`):

```bash
cp projects/P4-api-aggregator/_solution/*.go projects/P4-api-aggregator/starter/
go test ./projects/P4-api-aggregator/tests/
```
