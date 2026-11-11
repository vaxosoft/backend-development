# 07. HTTP Client in Rust

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 04, 06 (async basics)

## Learning Outcomes
After this module you can:
- [ ] Send requests with headers, JSON and bearer tokens, and decode JSON responses into typed structs
- [ ] Reuse one `reqwest::Client` and configure timeouts so a slow upstream cannot hang your service
- [ ] Treat non-2xx statuses, transport errors and decode errors as three different failures
- [ ] Retry idempotent calls with exponential backoff, and never retry 4xx
- [ ] Test client code offline against a local mock server (`wiremock`)

## 1. Concept Overview
An HTTP client is the mirror image of the server from module 02: you build a request (method, URL, headers, body), the library sends it over a pooled connection, and you get a response whose body you read **once**. Three things separate toy code from production code: **reuse** (connections are expensive, so one client lives for the whole process), **bounded time** (every call needs a timeout) and **explicit failure handling** (a `404` is a valid HTTP answer, not a network failure).

```
your code --Client (pool, timeouts, default headers)--> upstream
   ^                                                       |
   +---- Response {status, headers, body (read once)} <----+
```

Retries sit on top: if a call fails for a *transient* reason (connection refused, timeout, 5xx, 429) and repeating it is safe (idempotent: GET, PUT, DELETE), wait `base * 2^n` (capped, ideally with jitter) and try again.

## 2. How Rust Approaches It

| Piece | Role |
|-------|------|
| `reqwest::Client` | async client; internally an `Arc`, so `clone()` is cheap and shares the pool |
| `RequestBuilder` | `.header()`, `.bearer_auth()`, `.json(&body)`, `.timeout()`, `.send().await` |
| `Response` | `.status()`, `.headers()`, then ONE consuming call: `.json::<T>()`, `.text()`, `.bytes()` |
| `error_for_status()` | turns a 4xx/5xx `Response` into a `reqwest::Error` (when you do not need the body) |
| `thiserror` enum | your own error type; `#[from] reqwest::Error` keeps `?` ergonomic |
| `wiremock` | in-process mock server used by the exercise tests |

- **Idiomatic:** one client built with `Client::builder()` and stored in a struct (`TaskClient { http, base, token }`); a typed method per endpoint; a domain error enum.
- **Avoid:** `reqwest::get(url)` in hot paths (builds a fresh client every call), `unwrap()` on `send()`, and calling `.json()` on a response before checking the status.
- **Features:** this workspace enables reqwest's `json`, `rustls` and `stream` features only. `RequestBuilder::query()` and `.form()` are behind the `query`/`form` features, so the examples build query strings with `Url::query_pairs_mut()`.
- **Retry** is hand-written (about 20 lines, example 3.4) so you see exactly what it does; there is no retry crate in the profile.

## 3. Guided Examples
The examples start a tiny axum server on port 0 inside the same process, so they run offline and never touch a fixed port. (Skim the `spawn_upstream` function at the bottom of each file; the lesson is in `main`.)

### 3.1 GET and decode JSON
- **Goal:** one request, status, headers, typed body.
- **Run:** `cargo run -p m07-http-client --example 01_get_json`
- **Code:** [examples/01_get_json.rs](examples/01_get_json.rs)
- **Walkthrough:**
  ```rust
  let response = client.get(format!("{base}/tasks/1")).header("Accept", "application/json").send().await?;
  let task: Task = response.json().await?;   // consumes `response`
  ```
  `Task` declares only the fields it needs; unknown JSON fields (`due_date`, `owner_id`) are ignored. `.json()` takes `self` by value, which is why you read `status()` and `headers()` first.
- **Expected output:**
  ```
  status: 200 OK
  content-type: application/json
  Task { id: 1, title: "Write docs", status: "todo", tags: ["docs", "p1"] }
  #1 "Write docs" is todo with 2 tags
  text body: pong
  ```

### 3.2 POST, bearer auth and the three kinds of failure
- **Goal:** distinguish transport errors, non-2xx answers and decode errors.
- **Run:** `cargo run -p m07-http-client --example 02_errors_and_auth`
- **Code:** [examples/02_errors_and_auth.rs](examples/02_errors_and_auth.rs)
- **Walkthrough:**
  ```rust
  let response = client.post(url).bearer_auth(token).json(&NewTask { title }).send().await?; // transport errors stop here
  if status.is_success() { return Ok(response.json().await?); }
  let body: Value = response.json().await.unwrap_or(Value::Null);                          // the API's error envelope
  Err(ApiError::Api { status: status.as_u16(), code: ..., message: ... })
  ```
  `send()` only fails when no HTTP answer arrived (refused connection, timeout, TLS). A `401` or `422` is `Ok(response)`. `error_for_status()` is the short form that discards the body.
- **Expected output:**
  ```
  created #1 "Write docs"
  failed: api error 401: unauthorized (missing or invalid credentials)
  failed: api error 422: validation_failed (invalid task)
  error_for_status: Some(401) true
  transport error: connect=true
  ```

### 3.3 One client, timeouts, connection reuse
- **Goal:** configure the client once; see why it must be shared.
- **Run:** `cargo run -p m07-http-client --example 03_client_reuse_and_timeouts`
- **Code:** [examples/03_client_reuse_and_timeouts.rs](examples/03_client_reuse_and_timeouts.rs)
- **Walkthrough:** the builder sets `connect_timeout`, an overall `timeout`, a `user_agent` and `default_headers`. The upstream counts accepted TCP connections: five requests through one client use one connection, five throwaway clients use five. A request can override the client timeout with `.timeout(...)`. When it fires, `error.is_timeout()` is `true`.
- **Expected output:**
  ```
  one client, 5 requests -> 1 connection(s)
  new client per request -> 5 connection(s)
  echo: accept=text/plain user-agent=task-tracker-client/1.0
  slow call: timeout=true
  slow call with 2s budget: finally
  ```

### 3.4 Retry with exponential backoff
- **Goal:** retry what is transient, give up on what is permanent.
- **Run:** `cargo run -p m07-http-client --example 04_retry_backoff`
- **Code:** [examples/04_retry_backoff.rs](examples/04_retry_backoff.rs)
- **Walkthrough:**
  ```rust
  fn delay(&self, retry: u32) -> Duration {
      self.base.saturating_mul(2u32.saturating_pow(retry)).min(self.cap)
  }
  ```
  The loop classifies each outcome: success returns; 5xx/429, connect errors and timeouts sleep `delay(n)` and try again until `max_retries`; any other status returns `Permanent` immediately. Production code should also add jitter (multiply the delay by a random factor between 0.5 and 1.0) so many clients do not retry in lockstep, and should only retry idempotent requests.
- **Expected output:**
  ```
  GET /flaky (fails twice, then succeeds)
    attempt 1 failed (status 503 Service Unavailable); retrying in 5ms
    attempt 2 failed (status 503 Service Unavailable); retrying in 10ms
    -> 200 OK after 3 requests
  GET /missing (404 is permanent)
    -> not retryable: status 404 Not Found after 1 request
  GET /broken (always 500)
    attempt 1 failed (status 500 Internal Server Error); retrying in 5ms
    attempt 2 failed (status 500 Internal Server Error); retrying in 10ms
    attempt 3 failed (status 500 Internal Server Error); retrying in 15ms
    -> gave up after 4 attempts: status 500 Internal Server Error
  ```

### 3.5 A typed Task Tracker client
- **Goal:** the pieces together: `TaskClient` with base URL, token, pagination query and error envelope.
- **Run:** `cargo run -p m07-http-client --example 05_task_client`
- **Code:** [examples/05_task_client.rs](examples/05_task_client.rs)
- **Walkthrough:** `list(status, limit, offset)` builds `?status=done&limit=2&offset=0` with `Url::query_pairs_mut()` (percent-encoding included), sends the bearer token, and `decode()` maps any non-2xx to `ClientError::Api { status, code, message }` using the shared error envelope from the domain document.
- **Expected output:**
  ```
  created #3 "Ship it" [release]
  page 1: 2 of 3 (limit 2, offset 0)
  page 2: 1 of 3 (limit 2, offset 2)
  done: [(2, Done)]
  bad token -> 401 unauthorized: missing or invalid credentials
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `reqwest::get(url)` or `Client::new()` per request | Convenient, and it compiles; but each client has its own pool, so every call opens a new connection (example 3.3 shows 5 connections vs 1) | Build one `Client`, store it in your struct or `State`, `clone()` it freely (cheap `Arc`) |
| No timeout | reqwest has **no default overall timeout**: a stalled upstream hangs the task forever | `Client::builder().timeout(..).connect_timeout(..)`; override per request with `.timeout()` |
| Assuming `send().await?` means success | A `404`/`500` is `Ok(Response)` | Check `status()` or call `error_for_status()` before decoding |
| `response.json()` then `response.status()` | `json()`, `text()`, `bytes()` take `self` by value: "borrow of moved value" | Read status and headers first; consume the body last |
| `.query(&[...])` does not compile | `query`/`form` are optional cargo features, not enabled in this workspace | Enable the feature in your own project, or use `Url::query_pairs_mut()` as in 3.5 |
| `.text()`/`.json()` on an untrusted or huge body | They buffer the entire body in memory | Cap with the `Content-Length` header and read `chunk()`/`bytes_stream()` with a running limit |
| Retrying `POST` or `4xx` | A retry loop is easy to write and hard to scope | Retry only idempotent methods and only 5xx/429/transport errors; keep a max and a cap |
| Logging or returning `reqwest::Error` as is | Its `Display` includes the full URL, which may carry tokens in the query string | Map to your own error enum; use `error.without_url()` when you must forward it |

## 5. Coming From Other Languages
- **Go:** `http.Client` is also meant to be shared and has no default timeout. There is no `defer resp.Body.Close()`: dropping the `Response` releases the connection. Errors are `Result`, not `(resp, err)` with `resp.StatusCode` checks, but non-2xx is still not an error.
- **TypeScript:** like `fetch`, a 4xx/5xx does not reject; like `response.json()`, the body is read once. `error_for_status()` is Rust's `if (!res.ok) throw`.
- **Python:** `Client` is `requests.Session`; `error_for_status()` is `raise_for_status()`; timeouts are mandatory in practice just as with `requests`.
- **Java/C#:** `HttpClient` is a long-lived singleton for the same pooling reasons; `Client` plus `thiserror` plays the role of typed exceptions.
- **C++:** the connection is an RAII resource: dropping the client or response closes or returns it.

## 6. Exercises
All tests use `wiremock` on loopback; each runs in well under a second. Starters compile and fail with `todo!()`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Fetch one task | Easy | status mapping, typed errors, decode vs transport | `cargo test -p m07-ex1-get-task` |
| 2 | Retry with backoff | Medium | classify failures, capped exponential delay, attempt counting | `cargo test -p m07-ex2-retry-backoff` |
| 3 | Task API client | Stretch | auth headers, paging, error envelope, timeout | `cargo test -p m07-ex3-task-api-client` |

**Exercise 1: `get_task`** ([starter](exercises/ex1-get-task/src/lib.rs), [tests](exercises/ex1-get-task/tests/get_task.rs))
- **Task:** `GET {base}/tasks/{id}` and map 2xx, 404, other statuses, undecodable bodies and connection failures to `FetchError` variants.
- **Acceptance:** all six tests pass, including the trailing-slash base URL and "bad body is `Decode`, not `Transport`".
<details><summary>Hint</summary>Check the status before calling <code>.json()</code>; <code>reqwest::Error</code> has <code>is_decode()</code> but you can also separate the two by <em>where</em> the error came from (<code>send</code> vs <code>json</code>).</details>

**Exercise 2: `get_text` with retry** ([starter](exercises/ex2-retry-backoff/src/lib.rs), [tests](exercises/ex2-retry-backoff/tests/retry.rs))
- **Task:** implement `RetryPolicy::delay` (capped, no overflow) and `get_text`, which retries connect errors, timeouts, 5xx and 429 up to `max_retries`.
- **Acceptance:** exact request counts (`expect(n)` on the mock server), 404 never retried, `Exhausted { attempts: 3 }` after two retries, timeouts retried.
<details><summary>Hint</summary>Use <code>saturating_mul</code>/<code>saturating_pow</code> so a huge retry number cannot overflow; count attempts before deciding to sleep.</details>

**Exercise 3: `TaskApiClient`** ([starter](exercises/ex3-task-api-client/src/lib.rs), [tests](exercises/ex3-task-api-client/tests/client.rs))
- **Task:** one `reqwest::Client` with timeout and user agent; `create_task` (bearer token, JSON body, 201) and `list_all` (follows `limit`/`offset` pages until `total`); decode the error envelope into `ApiError::Api`; timeouts become `ApiError::Timeout`.
- **Acceptance:** headers and bodies matched by the mock server, three pages fetched exactly once each, an empty page stops the loop, non-JSON error bodies still carry the status.
<details><summary>Hint</summary>Implement <code>From&lt;reqwest::Error&gt; for ApiError</code> once (timeout / decode / other) and let <code>?</code> do the rest.</details>

Reference solutions: [solutions/](solutions/) (overlay onto `exercises/`).

## 7. Recap
- One long-lived `Client`, always with timeouts.
- `send()` succeeding means "an answer arrived"; check the status yourself, and read the body once.
- Wrap `reqwest::Error` in your own enum so callers can tell timeout, status and decode errors apart.
- Retry only transient failures of idempotent calls, with capped exponential backoff (plus jitter).
- Test clients against a local mock (`wiremock`, or a trait-based fake in module 12), never the real internet.

## 8. Self-check Questions
1. Why does `response.json::<T>().await` consume the response, and how does that shape the order of your code?
2. Your service calls an upstream that sometimes hangs. Which two settings protect you, and where do you configure them?
3. Which of these may be retried automatically, and why: `GET /tasks/1` returning 503, `POST /tasks` timing out, `GET /tasks/1` returning 404?
4. What is the difference between `Err(reqwest::Error)` from `send()` and an `Ok(Response)` with status 500?
5. Why is a mock server on `127.0.0.1:0` better than calling a public test API in a unit test?

## 9. Further Reading
- [reqwest documentation](https://docs.rs/reqwest/0.13) (Client, RequestBuilder, Response)
- [wiremock-rs](https://docs.rs/wiremock/0.6) for matchers and `up_to_n_times`
- AWS Architecture Blog, "Exponential Backoff and Jitter"

---
**Next:** [08 File and Stream I/O](../08-file-and-stream-io/README.md)
