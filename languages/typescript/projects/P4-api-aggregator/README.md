# P4 - API Aggregator

Build a small gateway: `GET /aggregate?ids=1,2,3` asks an upstream service for each id **concurrently** (bounded), survives slow and flaky upstream calls, and answers with one combined document.

**Modules exercised:** [06 Concurrency](../../modules/06-concurrency/README.md), [07 HTTP client](../../modules/07-http-client/README.md).

## Requirements

| Topic            | Rule                                                                                                                                                                                                                  |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Endpoint         | `GET /aggregate?ids=1,2,3`                                                                                                                                                                                            |
| Upstream call    | `GET {upstreamBaseUrl}/items/{id}` for every id, JSON body (global `fetch`)                                                                                                                                           |
| Concurrency      | at most `maxConcurrency` (default 4) upstream calls in flight; ids start as soon as a slot is free                                                                                                                    |
| Per-call timeout | each attempt is cancelled after `callTimeoutMs` (default 2000)                                                                                                                                                        |
| Retries          | up to `maxRetries` (default 2) retries on network error, timeout and 5xx; the delay before retry _n_ (0-based) is `backoffBaseMs * 2^n` (default 100 ms); **never** retry 4xx or an unparsable body                   |
| Order            | `results` and `errors` both follow the order of the requested ids                                                                                                                                                     |
| Success body     | `{"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}`                                                                                                                     |
| Partial failure  | still HTTP `200`                                                                                                                                                                                                      |
| Error texts      | `upstream status {code}`, `timeout`, `upstream unreachable`, `invalid upstream response`, `deadline exceeded`                                                                                                         |
| Bad input        | `400` `bad_request` (shared error envelope) when `ids` is missing/empty, has more than 50 entries, or has an entry that is not a non-negative integer (`1,a`, `1,,2`, `-1`, `1.5`); spaces around entries are ignored |
| Deadline         | the whole request gets `overallTimeoutMs` (default 10000); outstanding calls are cancelled, ids that never started never start, and unfinished ids are reported as `deadline exceeded`                                |
| Disconnect       | when the client goes away, outstanding upstream calls are cancelled                                                                                                                                                   |

Error envelope: `{"error":{"code":"bad_request","message":"..."}}`.

## Configuration

`buildApp(config)` takes a complete `Config` (see `starter/src/config.ts`). `src/main.ts` builds it from environment variables:

| Env var              | Default                 |
| -------------------- | ----------------------- |
| `ADDR`               | `127.0.0.1:8080`        |
| `UPSTREAM_BASE_URL`  | `http://127.0.0.1:9000` |
| `MAX_CONCURRENCY`    | `4`                     |
| `CALL_TIMEOUT_MS`    | `2000`                  |
| `MAX_RETRIES`        | `2`                     |
| `BACKOFF_BASE_MS`    | `100`                   |
| `OVERALL_TIMEOUT_MS` | `10000`                 |

## Run and test

All commands run from `languages/typescript/`.

```bash
ADDR=127.0.0.1:8080 UPSTREAM_BASE_URL=http://127.0.0.1:9000 \
  node projects/P4-api-aggregator/starter/src/main.ts
curl -i 'http://127.0.0.1:8080/aggregate?ids=1,2,3'

node --test "projects/P4-api-aggregator/tests/**/*.test.ts"
```

The tests start a real **fake upstream** on a loopback port (`tests/fake-upstream.ts`) that counts calls and concurrent calls and can be scripted to fail, stall or drop connections. They call your app with `app.inject()`, except the disconnect test, which uses a real listener on port 0. No network, a few seconds.

With a real upstream running (any server answering `GET /items/{id}`), a flaky run looks like this:

```text
$ curl 'http://127.0.0.1:8080/aggregate?ids=1,2,3,4'
{"results":[{"id":1,"data":{"id":1,"name":"item-1"}},{"id":2,"data":{"id":2,"name":"item-2"}}],"errors":[{"id":3,"error":"upstream status 500"},{"id":4,"error":"upstream status 404"}]}
$ curl 'http://127.0.0.1:8080/aggregate?ids=1,x'
{"error":{"code":"bad_request","message":"ids must be non-negative integers separated by commas"}}
```

## What to implement

The starter type-checks and is fully wired (`app.ts`, `main.ts`, config, types, the client-disconnect signal). Three `TODO(P4)` stubs are yours:

| File                        | Task                                                                        |
| --------------------------- | --------------------------------------------------------------------------- |
| `starter/src/ids.ts`        | parse and validate the `ids` query value                                    |
| `starter/src/upstream.ts`   | one item: per-attempt timeout, retry with backoff, error classification     |
| `starter/src/aggregator.ts` | bounded fan-out, ordering, partial failures, overall deadline, cancellation |

## Acceptance criteria -> tests (`tests/aggregate.test.ts`)

| Criterion                     | Test                                                                                                                                 |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| invalid `ids` -> 400 envelope | `missing ids returns 400 with the error envelope`, `empty, non-numeric, negative, fractional and blank entries return 400`           |
| at most 50 ids                | `more than 50 ids returns 400 and calls nothing`, `exactly 50 ids are accepted`                                                      |
| whitespace tolerated          | `spaces around ids are ignored`                                                                                                      |
| response shape                | `wraps the upstream JSON in data`                                                                                                    |
| request order preserved       | `keeps request order even when the upstream answers out of order`                                                                    |
| partial failure is 200        | `partial failure is still 200 with results and errors in request order`                                                              |
| unparsable upstream body      | `an invalid upstream JSON body is reported and not retried`                                                                          |
| concurrency bound             | `never has more than maxConcurrency calls in flight`, `defaults to 4 concurrent calls and really runs them in parallel`              |
| retry on 5xx, then give up    | `retries a 5xx and succeeds on the third attempt`, `gives up after two retries and reports the status`, `maxRetries is configurable` |
| exponential backoff           | `backs off exponentially between attempts`                                                                                           |
| never retry 4xx               | `does not retry 4xx`                                                                                                                 |
| network errors                | `retries dropped connections and recovers`, `reports upstream unreachable after repeated network errors`                             |
| per-call timeout              | `a per-call timeout is retried and finally reported as timeout`                                                                      |
| overall deadline cancels      | `the overall deadline cancels outstanding upstream calls`, `ids still waiting for a slot are never started after the deadline`       |
| client disconnect cancels     | `a client disconnect cancels outstanding upstream calls`                                                                             |

## Suggested steps and hints

1. `parseIds`, then run the validation tests.
2. `fetchItem` with a single attempt, then add the retry loop.
3. `aggregate`: first sequentially, then concurrently, then bounded.
4. Deadline and disconnect last.

<details>
<summary>Hint: validation</summary>

`raw.split(',')` keeps empty entries (good: `1,,2` must fail). Test each trimmed entry with `/^\d+$/`, which rejects signs, dots and blanks, and convert with `Number`.
</details>

<details>
<summary>Hint: per-attempt timeout</summary>

`AbortSignal.any([signal, AbortSignal.timeout(ms)])` gives every attempt its own timeout while still honouring the caller's cancellation. In the `catch`, check `signal.aborted` first (the caller gave up: stop), then `error.name === 'TimeoutError'` (this attempt was too slow: retry). `fetch` does not reject on 5xx: look at `response.status`, and cancel the body (`await response.body?.cancel()`) when you do not read it.
</details>

<details>
<summary>Hint: waiting between retries</summary>

`import { setTimeout as sleep } from 'node:timers/promises'` and pass `{ signal }` so a cancelled request does not sit out its backoff.
</details>

<details>
<summary>Hint: bounding concurrency</summary>

Start `Math.min(maxConcurrency, ids.length)` workers that pull from one shared iterator (`const queue = ids.entries()`; each worker runs `for (const [index, id] of queue)`). Write each outcome to `outcomes[index]`, so order never depends on completion order. `Promise.all` waits for the workers. An `await` inside a plain `for` loop would be sequential, and `Promise.all(ids.map(...))` alone would be unbounded.
</details>

<details>
<summary>Hint: deadline vs disconnect</summary>

`AbortSignal.any([clientSignal, AbortSignal.timeout(overallTimeoutMs)])` is one signal for both. Check `signal.aborted` before starting the next id, and pre-fill every outcome with `DEADLINE_EXCEEDED`. When the client left nobody reads the answer, so the same path is fine.
</details>

## Stretch goals (not tested)

- Add jitter to the backoff.
- Cache successful items for a few seconds (inject the clock so it stays testable).
- Honour an upstream `Retry-After` header.
- Add `GET /healthz` that checks the upstream with a short timeout.

## Reference solution

`solution/src/` holds only the three files that differ from `starter/src/`. Try the tasks first, then compare. To run the tests against it, save your own work first, then overlay it: `cp -r projects/P4-api-aggregator/solution/src/. projects/P4-api-aggregator/starter/src/`.
