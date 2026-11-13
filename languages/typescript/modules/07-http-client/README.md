# 07. HTTP Client in TypeScript

> Estimated time: 1.5-2 h | Difficulty: Intermediate | Prerequisites: 01 (promises, errors), 04 (JSON, zod), 06 (`AbortSignal`)

## Learning Outcomes

After this module you can:

- [ ] Make GET/POST requests with the global `fetch`, set headers and query strings, and round-trip JSON
- [ ] Treat non-2xx responses and network failures as different kinds of errors (`res.ok`, `HttpError`, `TypeError`)
- [ ] Put an explicit deadline on every request and cancel it with `AbortSignal.timeout` / `AbortController`
- [ ] Retry transient failures with exponential backoff without retrying non-idempotent requests
- [ ] Wrap a third-party API in a small typed client with auth headers and validated responses
- [ ] Test client code offline against a loopback `node:http` server or an injected `fetch`

## 1. Concept Overview

An HTTP client turns a function call into a network exchange that can fail in ways a local call cannot: the host may be unreachable, the server may stall, or it may answer with an error status. A robust client therefore has four habits: **always set a timeout**, **classify failures** (network error, HTTP error, bad payload), **retry only what is safe and likely to heal** (idempotent requests, 5xx/429, with growing and jittered delays) and **reuse connections** instead of opening one per call.

```
caller -> client wrapper -> fetch -> [timeout | abort signal] -> server
            |   ^ retry/backoff, auth header, base URL
            v   |
      typed result / ApiError / TimeoutError
```

Third-party APIs add concerns: authentication headers, pagination, rate limits (`429` + `Retry-After`), and responses you must not trust until validated.

## 2. How TypeScript Approaches It

| Piece                                                                  | Use                                                             |
| ---------------------------------------------------------------------- | --------------------------------------------------------------- |
| global `fetch` (undici, built into Node 24+)                           | the client; returns a `Response` whose body is a web stream     |
| `Request`, `Response`, `Headers`, `URL`, `URLSearchParams`             | web-standard types, identical to the browser                    |
| `AbortSignal.timeout(ms)`, `AbortSignal.any([...])`, `AbortController` | deadlines and cancellation                                      |
| `node:timers/promises` `setTimeout`                                    | awaitable delay for backoff (accepts a `signal`)                |
| `zod`                                                                  | validate what a third party sends (types are erased at runtime) |
| `node:http`                                                            | a loopback fake server in examples and tests                    |

Idiomatic style: a small class or function that owns the base URL, auth, timeout and error mapping, takes `fetch` as an optional dependency (`typeof fetch`) and returns **validated, typed values**. Avoid sprinkling raw `fetch(...)` calls and `as Foo` casts across the code base. No axios/got is needed ([PROFILE](../../PROFILE.md)); retry and backoff are a few lines you write and test yourself.

`fetch` keeps connections alive in a shared pool, so you get connection reuse for free as long as you **consume or cancel every response body**.

## 3. Guided Examples

Run all commands from `languages/typescript/`. Every example starts its own loopback server on a free port (see [`_shared.ts`](examples/_shared.ts)), so no network is needed.

### 3.1 Fetch basics

- **Goal:** GET and POST with headers, query string and a JSON round-trip.
- **Run:** `node modules/07-http-client/examples/01_fetch_basics.ts`
- **Code:** [`examples/01_fetch_basics.ts`](examples/01_fetch_basics.ts)

```ts
const url = new URL('/search', server.url);
url.searchParams.set('q', 'a b&c');
const res = await fetch(url, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ title: 'Write docs', tags: ['docs'] }),
});
const echoed: unknown = await res.json();
```

- **Walkthrough:** `URLSearchParams` does the encoding (`a b&c` becomes `a+b%26c`). `fetch` labels a string body `text/plain`, so set `Content-Type: application/json` yourself. `res.json()` is typed `Promise<any>`; assigning it to `unknown` forces you to validate before use.
- **Expected output:**

  ```
  200 OK true
  application/json
  {
    method: 'GET',
    path: '/hello?name=ada',
    accept: 'application/json',
    contentType: null,
    body: null
  }
  {
    method: 'POST',
    path: '/search?q=a+b%26c',
    accept: '*/*',
    contentType: 'application/json',
    body: { title: 'Write docs', tags: [ 'docs' ] }
  }
  ```

### 3.2 Errors and status codes

- **Goal:** `fetch` resolves on 404/500; only network failures reject.
- **Run:** `node modules/07-http-client/examples/02_errors_and_status.ts`
- **Code:** [`examples/02_errors_and_status.ts`](examples/02_errors_and_status.ts)

```ts
const res = await fetch(url);
if (!res.ok) {
  throw new HttpError(res.status, await res.text());
}
return res.json();
```

- **Walkthrough:** the naive call prints `404 false` without throwing. `HttpError` carries status and body so callers can branch on it. Reading (or `cancel()`ing) the body releases the connection. A refused connection rejects with `TypeError: fetch failed` whose `cause.code` is `ECONNREFUSED`.
- **Expected output:**

  ```
  fetch resolved: 404 false
  { id: 1, title: 'Write docs' }
  /tasks/99 -> 404 {"error":{"code":"not_found","message":"no such task"}}
  /boom -> 500 {"error":{"code":"internal","message":"unexpected error"}}
  fetch failed / ECONNREFUSED
  ```

### 3.3 Timeouts and cancellation

- **Goal:** bound every request and tell timeouts from user cancellation.
- **Run:** `node modules/07-http-client/examples/03_timeouts_and_abort.ts`
- **Code:** [`examples/03_timeouts_and_abort.ts`](examples/03_timeouts_and_abort.ts)

```ts
await fetch(url, { signal: AbortSignal.timeout(50) }); // TimeoutError
await fetch(url, { signal: controller.signal }); // AbortError
const either = AbortSignal.any([caller.signal, AbortSignal.timeout(500)]);
```

- **Walkthrough:** the server never answers. `AbortSignal.timeout` rejects with a `DOMException` named `TimeoutError`; `controller.abort()` gives `AbortError`; `controller.abort(reason)` rejects with your own reason. `AbortSignal.any` merges a per-request timeout with a caller-owned signal (for example "the incoming request was cancelled").
- **Expected output:**

  ```
  timeout -> TimeoutError
  user abort -> AbortError
  abort reason -> shutting down
  ```

### 3.4 Retry with exponential backoff

- **Goal:** retry only idempotent requests, only on failures that can heal.
- **Run:** `node modules/07-http-client/examples/04_retry_backoff.ts`
- **Code:** [`examples/04_retry_backoff.ts`](examples/04_retry_backoff.ts)

```ts
const maxRetries = IDEMPOTENT.has(method) ? options.retries : 0;
...
const delayMs = Math.round(jitter() * Math.min(cap, base * 2 ** attempt));
```

- **Walkthrough:** `503` twice then `200` succeeds after three calls. `404` is returned at once (retrying cannot help). A persistent `500` is retried three times and the last response is returned. `POST` is never retried because a repeated POST may create two tasks. The example passes `jitter: () => 1` to make delays predictable; in production leave the default random factor so many clients do not retry in lockstep. The failed response body is cancelled before sleeping.
- **Expected output:**

  ```
  GET /flaky
    retry #1 in 10ms (status 503)
    retry #2 in 20ms (status 503)
    final status 200 after 3 call(s)
  GET /missing
    final status 404 after 1 call(s)
  GET /always-down
    retry #1 in 10ms (status 500)
    retry #2 in 20ms (status 500)
    retry #3 in 40ms (status 500)
    final status 500 after 4 call(s)
  POST /always-down
    final status 500 after 1 call(s)
  ```

### 3.5 A typed Task Tracker client

- **Goal:** one class that owns base URL, bearer auth, timeout, error envelope and response validation.
- **Run:** `node modules/07-http-client/examples/05_task_tracker_client.ts`
- **Code:** [`examples/05_task_tracker_client.ts`](examples/05_task_tracker_client.ts)

```ts
const res = await doFetch(new URL(path, this.#options.baseUrl), {
  method,
  headers: { Accept: 'application/json', Authorization: `Bearer ${token}` },
  signal: AbortSignal.timeout(this.#options.timeoutMs ?? 5000),
});
const payload: unknown = await res.json();
if (!res.ok) { /* parse { error: { code, message } } into ApiError */ }
return schema.parse(payload);
```

- **Walkthrough:** the shared error envelope becomes an `ApiError` with `status` and `code`; success payloads go through a zod schema so a server change fails loudly at the boundary. `fetch` is injectable (`options.fetch`), which is the cheap way to unit-test the client without a server.
- **Expected output:**

  ```
  created {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    due_date: null,
    tags: [ 'docs' ],
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z'
  }
  fetched Write docs
  list total 1
  ApiError 404 not_found: task not found
  ApiError 401 unauthorized: missing or invalid token
  ```

## 4. Common Pitfalls

| Mistake                                          | Why it happens                                                                   | Fix                                                                                    |
| ------------------------------------------------ | -------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| Assuming `await fetch()` throws on 404/500       | the Fetch spec only rejects on network errors                                    | check `res.ok` (or `res.status`) and throw your own `HttpError`                        |
| No timeout                                       | undici only gives up after about 5 minutes (headers/body timeouts)               | pass `signal: AbortSignal.timeout(ms)` on every call                                   |
| Never reading or cancelling the body             | the connection stays checked out of the pool and memory is held                  | `await res.text()/json()` or `await res.body?.cancel()` even for responses you discard |
| `const data: Task = await res.json()`            | `json()` returns `any`, so the annotation is a lie                               | assign to `unknown` and validate with zod                                              |
| Retrying `POST` (or any non-idempotent call)     | the first attempt may have reached the server                                    | retry only GET/HEAD/PUT/DELETE, or use an idempotency key                              |
| Retrying instantly in a tight loop               | no delay, no cap, no jitter                                                      | exponential backoff with a cap and random jitter; honour `Retry-After`                 |
| Confusing the abort errors                       | timeout is `TimeoutError`, manual abort is `AbortError`, both are `DOMException` | compare `err.name`, or check `signal.aborted` to see who gave up                       |
| Building URLs with string concatenation          | missing slashes, unencoded query values                                          | `new URL(path, base)` and `url.searchParams.set(...)`                                  |
| Treating `TypeError: fetch failed` as a code bug | undici wraps socket errors in a generic `TypeError`                              | inspect `err.cause` (`code`: `ECONNREFUSED`, `ECONNRESET`, `UND_ERR_*`)                |
| Forwarding user input as a full URL              | SSRF: the server fetches internal addresses                                      | build URLs from a fixed base plus validated path segments                              |

## 5. Coming From Other Languages

- **JavaScript (browser):** the same `fetch`, `Headers` and `AbortSignal` API. Node adds no cookie jar and no CORS; `fetch` follows redirects by default.
- **Java / C#:** `fetch` is like a shared `HttpClient`: one pool per process, not one client per request. There is no per-client config object; timeouts and headers are per call, so a wrapper class is where defaults live.
- **Python:** like `httpx`, but `raise_for_status()` does not exist: you write the `res.ok` check. The default is _no_ timeout (httpx defaults to 5 s).
- **Go:** like `http.Client` plus `context.WithTimeout`: the `AbortSignal` plays the role of the `context`.

## 6. Exercises

Starters live in [`exercises/`](exercises/), reference solutions in [`solutions/`](solutions/). Run from `languages/typescript/`. To check a solution, copy its file over the starter (and restore the starter afterwards).

| #   | Title                                                              | Difficulty | Skill practiced                                   | Run tests                                                                    |
| --- | ------------------------------------------------------------------ | ---------- | ------------------------------------------------- | ---------------------------------------------------------------------------- |
| 1   | [Get JSON](exercises/ex1_get_json/get-json.ts)                     | Easy       | URLs, headers, `res.ok`, `HttpError`              | `node --test "modules/07-http-client/exercises/ex1_get_json/*.test.ts"`      |
| 2   | [Fetch with timeout](exercises/ex2_fetch_timeout/fetch-timeout.ts) | Medium     | `AbortSignal.timeout/any`, error classification   | `node --test "modules/07-http-client/exercises/ex2_fetch_timeout/*.test.ts"` |
| 3   | [Fetch with retry](exercises/ex3_fetch_retry/fetch-retry.ts)       | Stretch    | backoff, idempotency, `Retry-After`, cancellation | `node --test "modules/07-http-client/exercises/ex3_fetch_retry/*.test.ts"`   |

**Exercise 1 - Get JSON**

- **Task:** implement `buildUrl` (join base + path, add query, skip `undefined`) and `getJson` (Accept and bearer headers, parsed body, `HttpError` on non-2xx).
- **Acceptance:** all tests in `get-json.test.ts` pass.
- <details><summary>Hint</summary>Strip duplicate slashes yourself, then let `URL.searchParams` encode the query.</details>

**Exercise 2 - Fetch with timeout**

- **Task:** implement `fetchWithTimeout` so that a stalled server yields a `RequestTimeoutError`, while a caller abort or a connection failure passes through unchanged.
- **Acceptance:** all tests in `fetch-timeout.test.ts` pass (the stalled server is simulated on loopback; each test takes milliseconds).
- <details><summary>Hint</summary>Keep a reference to the timeout signal and check `timeout.aborted` (and `options.signal?.aborted`) in the `catch` to decide which error to throw.</details>

**Exercise 3 - Fetch with retry**

- **Task:** implement `fetchWithRetry`: backoff `base * 2 ** n`, retry on network errors and 429/5xx for idempotent methods only, use `Retry-After`, cancel discarded bodies and stop when the caller aborts.
- **Acceptance:** all tests in `fetch-retry.test.ts` pass; delays are recorded through an injected `sleep`, so no test really waits.
- <details><summary>Hint</summary>A `for (let attempt = 0; ; attempt++)` loop where each iteration either returns/throws or falls through to `await sleep(...)` keeps the control flow flat.</details>

## 7. Recap

- `fetch` rejects only on network failure: **you** turn non-2xx into errors, and you must read or cancel every body.
- Always pass a deadline (`AbortSignal.timeout`) and let callers add their own cancellation with `AbortSignal.any`.
- Retry only idempotent requests on 429/5xx/network errors, with exponential backoff, a cap, jitter and `Retry-After`.
- Wrap third-party APIs in one typed client that validates responses and takes `fetch` as a dependency, so tests stay offline.
- Reach for a hand-written wrapper over `fetch` first; add a library only when you need features such as interceptors or HTTP/2 tuning.

## 8. Self-check Questions

1. A call returns `res.status === 503` but no `catch` block ran. Why, and what does that imply for your retry loop?
2. Why is retrying a `POST /tasks` after a timeout dangerous, and what would make it safe?
3. What is the difference between `AbortSignal.timeout(100)` and `setTimeout(() => controller.abort(), 100)` for the error you catch?
4. What happens to the connection pool if you discard a `Response` without reading its body?
5. Why does the example client accept `fetch` as an option rather than importing a mocking library?

## 9. Further Reading

- [MDN: Fetch API](https://developer.mozilla.org/docs/Web/API/Fetch_API) and [`AbortSignal`](https://developer.mozilla.org/docs/Web/API/AbortSignal)
- [Node.js docs: `fetch`, undici and global dispatcher](https://nodejs.org/api/globals.html#fetch)
- [AWS Architecture Blog: Exponential Backoff and Jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/)
- [RFC 9110 section 9.2.2: idempotent methods](https://www.rfc-editor.org/rfc/rfc9110#name-idempotent-methods) and `Retry-After`

---

**Next:** [08 File and Stream I/O](../08-file-and-stream-io/README.md)
