# 07. HTTP Client in C#

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 04, 06

## Learning Outcomes
After this module you can:
- [ ] Send GET/POST requests with headers and JSON bodies and decode typed responses.
- [ ] Treat non-2xx responses deliberately (`IsSuccessStatusCode`, `EnsureSuccessStatusCode`, "not found means null").
- [ ] Reuse connections correctly with `IHttpClientFactory` and typed clients.
- [ ] Add timeouts and retries with exponential backoff (never retrying 4xx), and cancel outstanding calls.
- [ ] Test code that calls HTTP with a fake `HttpMessageHandler`, without a network.

## 1. Concept Overview
An HTTP client call is a remote procedure call that can fail in ways a local call cannot: the connection is refused, the server hangs, it answers `503`, or it answers `200` with a body you did not expect. A production client therefore needs four things: **connection reuse** (TCP/TLS handshakes are expensive), **deadlines** (every call must be able to give up), **a policy for failure** (retry only what is safe and transient, with backoff) and **a seam for tests** (swap the transport, not the logic).

```
caller -> typed client -> [handler: retry/timeout] -> [handler: headers] -> SocketsHttpHandler -> network
                              (DelegatingHandler pipeline, outermost first)
```

## 2. How C# Approaches It

| Piece | Use |
|-------|-----|
| `HttpClient` | Thread-safe, meant to be long-lived and shared. Sends `HttpRequestMessage`, returns `HttpResponseMessage`. |
| `System.Net.Http.Json` | `PostAsJsonAsync`, `GetFromJsonAsync`, `ReadFromJsonAsync<T>` (System.Text.Json under the hood). |
| `IHttpClientFactory` | Pools and recycles the underlying handlers (no socket exhaustion, DNS changes picked up). `AddHttpClient<TClient>()` registers a **typed client**. |
| `DelegatingHandler` | The middleware of the client side: wrap every request (auth headers, logging, retry). |
| `Microsoft.Extensions.Http.Resilience` | Polly v8 pipelines on the handler chain: `AddResilienceHandler` (custom) or `AddStandardResilienceHandler` (retry + circuit breaker + timeouts with sane defaults). |
| `HttpMessageHandler` fake | Subclass it (or `DelegatingHandler`) and return canned responses: the idiomatic test double. |

Idiomatic: inject `HttpClient` into a typed client class, pass a `CancellationToken`, dispose each `HttpResponseMessage`, own the retry policy in the handler pipeline. Non-idiomatic: `new HttpClient()` per call, `using var client` around a shared instance, hand-rolled `for` retry loops in business code, `Thread.Sleep`.

## 3. Guided Examples
Every example starts a throw-away upstream on `127.0.0.1` (free port, or `ADDR` if set), so it runs offline and exits by itself. Run from `languages/csharp/`; the dispatcher takes the example name: `dotnet run --project modules/07-http-client/examples -- <name>`.

### 3.1 Status codes are data until you ask for an exception
- **Goal:** know when `HttpClient` throws and when it does not.
- **Run:** `dotnet run --project modules/07-http-client/examples -- 01-get-and-status`
- **Code:** [examples/01_get_and_status.cs](examples/01_get_and_status.cs)
- **Walkthrough:** `GetAsync` returns normally for `404`/`503`; only network failures and timeouts throw. `EnsureSuccessStatusCode()` converts a bad status into `HttpRequestException` (with `StatusCode`). The `GetStringAsync`/`GetStreamAsync` shortcuts call it for you.
- **Expected output:**
  ```
  GET /hello   -> 200 OK, success=True
  content-type -> text/plain; charset=utf-8
  body         -> hello from upstream
  GET /missing -> 404 Not Found, success=False
  GET /boom    -> EnsureSuccessStatusCode threw, StatusCode=503
  GetStringAsync(/missing) threw, StatusCode=404
  ```

### 3.2 JSON bodies and per-request headers
- **Goal:** round-trip a DTO and send a Bearer token without mutating shared state.
- **Run:** `dotnet run --project modules/07-http-client/examples -- 02-json-and-headers`
- **Code:** [examples/02_json_and_headers.cs](examples/02_json_and_headers.cs)
- **Walkthrough:** `PostAsJsonAsync(url, dto, options)` serializes with snake_case options; `ReadFromJsonAsync<TaskItem>` decodes. A credential that differs per call goes on the `HttpRequestMessage` (`request.Headers.Authorization = new("Bearer", token)`), never on `DefaultRequestHeaders` of a shared client. (`created_at` prints `+00:00`, which is valid RFC 3339 for UTC.)
- **Expected output:**
  ```
  POST /tasks -> 201
  decoded     -> #1 'Write docs' Todo due 2030-01-31 tags=[docs]
  raw body    -> {"id":2,"title":"Second","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00+00:00"}
  GET /me without token -> 401
  GET /me with token    -> 200 {"name":"Ada","accept":"application/json"}
  ```

### 3.3 `IHttpClientFactory`, typed clients and handlers
- **Goal:** register a client once, get pooled connections, and add cross-cutting behaviour with a `DelegatingHandler`.
- **Run:** `dotnet run --project modules/07-http-client/examples -- 03-typed-client`
- **Code:** [examples/03_typed_client.cs](examples/03_typed_client.cs)
  ```csharp
  services.AddHttpClient<WhoAmIClient>(c => { c.BaseAddress = server.BaseAddress; c.DefaultRequestHeaders.Add("X-Api-Key", "demo-key"); })
          .AddHttpMessageHandler<StampHandler>()
          .SetHandlerLifetime(TimeSpan.FromMinutes(2));
  ```
- **Walkthrough:** the `HttpClient` instances are cheap and short-lived; the factory keeps the expensive handler chain alive for the handler lifetime and then recycles it (so DNS changes are honoured). `StampHandler` runs for every request.
- **Expected output:**
  ```
  call 1: api_key=demo-key, stamped_by=StampHandler, user_agent=task-tracker-cli/1.0
  call 2: api_key=demo-key, stamped_by=StampHandler, user_agent=task-tracker-cli/1.0
  ```

### 3.4 Retries, backoff and timeouts
- **Goal:** retry transient failures only, bound every attempt, and let the caller cancel.
- **Run:** `dotnet run --project modules/07-http-client/examples -- 04-retry-and-timeout`
- **Code:** [examples/04_retry_and_timeout.cs](examples/04_retry_and_timeout.cs)
  ```csharp
  .AddResilienceHandler("retry", pipeline =>
  {
      pipeline.AddRetry(new HttpRetryStrategyOptions { MaxRetryAttempts = 2, Delay = 5ms, BackoffType = DelayBackoffType.Exponential, UseJitter = false });
      pipeline.AddTimeout(TimeSpan.FromMilliseconds(200)); // per attempt: it sits inside the retry
  });
  ```
- **Walkthrough:** `HttpRetryStrategyOptions` retries 5xx, 408, 429 and network errors by default, not other 4xx. Order matters: the timeout added after the retry limits each attempt. The caller's `CancellationToken` stops the whole pipeline (real services keep jitter on; it is off here for reproducibility).
- **Expected output:**
  ```
  /flaky        -> 200 after 3 attempt(s)
  /bad-request  -> 400 after 1 attempt(s)
  /always-down  -> 500 after 3 attempt(s)
  /hangs-once   -> 200 after 2 attempt(s)
  caller deadline -> OperationCanceledException
  ```

### 3.5 A Task Tracker API client (real server and stub)
- **Goal:** a typed client that maps HTTP outcomes to a domain API, streams a big response, and is testable.
- **Run:** `dotnet run --project modules/07-http-client/examples -- 05-task-tracker-client`
- **Code:** [examples/05_task_tracker_client.cs](examples/05_task_tracker_client.cs)
- **Walkthrough:** `GetAsync` maps `404` to `null`; other failures become `TaskTrackerException` built from the `application/problem+json` body. `StreamExportAsync` uses `HttpCompletionOption.ResponseHeadersRead` and reads NDJSON line by line, so memory stays flat. The last lines build the same client over a `StubHandler` (an `HttpMessageHandler` returning a canned `404`).
- **Expected output:**
  ```
  created #1: Write docs (Todo)
  get #1  -> Write docs
  get #99 -> null (404 mapped to null)
  invalid -> 422: title must be 1-200 characters
  export line: {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00+00:00"}
  export line: {"id":2,"title":"Ship it","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00+00:00"}
  stubbed 404 -> True
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `new HttpClient()` per request (or `using var c = new HttpClient()` in a method) | It looks like a disposable, so people dispose it; each instance owns a connection pool | One shared instance or `IHttpClientFactory`; never dispose a shared client |
| A static `HttpClient` that never sees DNS changes | Pooled connections live forever | Use the factory (handler lifetime) or `SocketsHttpHandler.PooledConnectionLifetime` |
| Assuming a `404`/`500` throws | Other languages' clients do; `HttpClient` only throws on transport failure | Check `IsSuccessStatusCode` or call `EnsureSuccessStatusCode` |
| `ReadAsStringAsync()` on a huge body, `GetAsync` with the default buffering | The default `ResponseContentRead` downloads everything first | `HttpCompletionOption.ResponseHeadersRead` + `ReadAsStreamAsync` |
| Not disposing `HttpResponseMessage` | It holds the connection until the body is read or the response is disposed | `using var response = ...` |
| Mutating `DefaultRequestHeaders` of a shared client per request (auth tokens) | It is global state, racy across threads | Put per-call headers on the `HttpRequestMessage` or in a handler |
| Catching `TaskCanceledException` as "timeout" only | `HttpClient.Timeout` surfaces as `TaskCanceledException` with a `TimeoutException` inner, same type as caller cancellation | Check `cancellationToken.IsCancellationRequested` (or use resilience timeouts, which throw `TimeoutRejectedException`) |
| Retrying `POST` blindly, or re-sending a consumed request | Non-idempotent calls duplicate effects; request content can be sent only once | Use `DisableForUnsafeHttpMethods()` or idempotency keys; buffer content (`LoadIntoBufferAsync`) in custom handlers |
| `.Result` / `.Wait()` on client calls | Sync-over-async blocks threads and can deadlock | `await` all the way |

## 5. Coming From Other Languages
- **Go:** `http.Client` is also shared and reusable, but non-2xx is likewise not an error; C# adds `EnsureSuccessStatusCode` and a handler pipeline instead of `RoundTripper` wrapping.
- **Java:** `HttpClient` (java.net.http) is immutable and built with a builder; here the factory plus `DelegatingHandler` plays the role of OkHttp/Spring interceptors and `RestClient` configuration.
- **TypeScript/JS:** `fetch` also resolves on `404`; C# `response.EnsureSuccessStatusCode()` is the usual `if (!res.ok) throw`. `AbortController` ~ `CancellationToken`.
- **Python:** `requests.Session` ~ a shared `HttpClient`; `raise_for_status()` ~ `EnsureSuccessStatusCode()`; `tenacity`/`urllib3.Retry` ~ the resilience handler.

## 6. Exercises
Starters compile and fail; tests are in the same project. Run one exercise at a time from `languages/csharp/`. Reference solutions are in [solutions/](solutions/) (copy a file over its starter to check).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Status handling | Easy | Map statuses to results, dispose responses, send `Accept` | `dotnet test modules/07-http-client/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Retry handler | Medium | Write a `DelegatingHandler` with backoff, buffered content, cancellation | `dotnet test modules/07-http-client/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Resilient client factory | Stretch | Wire a typed client, default headers and a resilience pipeline with `IHttpClientFactory` | `dotnet test modules/07-http-client/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: [Ex1_StatusHandling.cs](exercises/Ex1_StatusHandling.cs)
- **Task:** implement `TaskTitleClient.GetTitleAsync`: `200` returns the `title`, `404` returns `null`, anything else throws `HttpRequestException` with the status.
- **Acceptance:** `Ex1_StatusHandlingTests` (request URL and `Accept` header, status mapping, response disposed on every path, cancellation).
- **Hint:** <details><summary>Show</summary>Use `SendAsync(request, ct)` inside `using var response`, test `StatusCode == NotFound` before `EnsureSuccessStatusCode()`.</details>

### Exercise 2: [Ex2_RetryHandler.cs](exercises/Ex2_RetryHandler.cs)
- **Task:** implement `RetryHandler.SendAsync`: retry 5xx/408/429/`HttpRequestException`/timeouts up to `maxRetries` with `100 ms * 2^n` backoff through the injected `delay`; never retry other 4xx or caller cancellation; re-send request bodies.
- **Acceptance:** `Ex2_RetryHandlerTests` (attempt counts, recorded delays, final response vs rethrow, body re-sent with a one-shot content).
- **Hint:** <details><summary>Show</summary>A one-shot body can only be re-sent if you buffered it first: `request.Content.LoadIntoBufferAsync()`. Dispose the response you are about to discard.</details>

### Exercise 3: [Ex3_ResilientClientFactory.cs](exercises/Ex3_ResilientClientFactory.cs)
- **Task:** implement `InventoryClientFactory.Build` so a typed `InventoryClient` has a base address, an `X-Api-Key` header, an injectable primary handler, a 2-retry constant 1 ms pipeline and a per-attempt timeout.
- **Acceptance:** `Ex3_ResilientClientFactoryTests` (base address and header, retry on 503/network error, retries stop after 2, 404 not retried, hanging attempt is timed out and retried).
- **Hint:** <details><summary>Show</summary>`AddHttpClient<T>(configure).ConfigurePrimaryHttpMessageHandler(...).AddResilienceHandler(name, p => { p.AddRetry(...); p.AddTimeout(...); })`; retry first so the timeout applies per attempt.</details>

## 7. Recap
- `HttpClient` is shared and long-lived; get it from `IHttpClientFactory` (typed client) in applications.
- A response status is data; decide per call what `404`, `401`, `5xx` mean, and dispose every response.
- Put retry/timeout policy in the handler pipeline (`AddResilienceHandler`), retry only transient failures, and keep callers' `CancellationToken` flowing.
- Test with a fake `HttpMessageHandler`; reserve loopback servers for integration-style checks.
- When to reach for what: a typed client + resilience handler for service calls; a bare `HttpClient` for a one-off script.

## 8. Self-check Questions
1. Why can `using var client = new HttpClient()` in a hot path make a service fail under load, and what does the factory change?
2. `GetAsync("x")` returns `500`. Does it throw? Which call turns that into an exception and what does it carry?
3. A retry strategy wraps a timeout strategy versus the other way around: how does the observable behaviour of a hanging server differ?
4. Why must a custom retry handler buffer the request content, and why is blind retry of `POST` risky?
5. How do you tell a client-side timeout from the caller cancelling the request?

## 9. Further Reading
- [HttpClient guidelines](https://learn.microsoft.com/dotnet/fundamentals/networking/http/httpclient-guidelines) and [IHttpClientFactory](https://learn.microsoft.com/dotnet/core/extensions/httpclient-factory)
- [Build resilient HTTP apps](https://learn.microsoft.com/dotnet/core/resilience/http-resilience) (`Microsoft.Extensions.Http.Resilience`)
- [Polly documentation](https://www.pollydocs.org/)

---
**Next:** [08 File and Stream I/O](../08-file-and-stream-io/README.md)
