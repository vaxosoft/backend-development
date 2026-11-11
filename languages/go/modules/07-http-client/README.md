# 07. HTTP Client in Go

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 04, 06

## Learning Outcomes
After this module you can:
- [ ] Make requests with headers, query strings and JSON bodies using `net/http`, and decode JSON responses.
- [ ] Configure timeouts (client, transport and per-request context) and explain which error each produces.
- [ ] Reuse connections by sharing one `http.Client` and always draining and closing the response body.
- [ ] Treat non-2xx answers as data: turn them into typed errors (`errors.As` / `errors.Is`).
- [ ] Retry with exponential backoff, only for idempotent requests and only for retryable failures.
- [ ] Test client code offline with `httptest.NewServer` and with a fake `http.RoundTripper`.

## 1. Concept Overview
An HTTP client turns "call that service" into a function call that can fail in many more ways than a local one: the network may be down, the peer may be slow, the answer may be a valid HTTP response that still means failure (`404`, `503`). Robust clients therefore always have: a **deadline**, a way to **classify failures** (retry or give up), **connection reuse** for speed, and a **seam for testing**.

```
caller -> request(ctx, headers, body) -> [client: timeout, retry, pooling] -> network -> service
caller <- typed result or typed error <-----------------------------------------------'
```

## 2. How Go Approaches It
| Piece | Role |
|-------|------|
| `http.Client` | policy: `Timeout`, redirects, cookie jar. Safe for concurrent use; create **once** and share |
| `http.Transport` | mechanism: connection pool, TLS, `ResponseHeaderTimeout`, `MaxIdleConnsPerHost` |
| `http.NewRequestWithContext` | the only request constructor you should use: it carries the `context.Context` (deadline, cancel) |
| `resp.Body` | a stream that you must `Close()` (and drain to reuse the connection) |
| `net/url` (`url.Values`) | correct query-string encoding |
| `httptest.NewServer` | a real loopback server for tests; `http.RoundTripper` fakes need no network at all |

Idiomatic style: small helper (`getJSON`, `do`) around `client.Do`, errors wrapped with `%w`, `ctx` as first parameter. Non-idiomatic: `http.Get` / `http.DefaultClient` in production code (no timeout), `ioutil`-style `ReadAll` of unbounded bodies, string-concatenated URLs. No third-party client library is needed; retries are ~20 lines you write yourself (see 3.4).

## 3. Guided Examples
All examples start their own `httptest` server, so they run offline and print deterministic output. Run them from `languages/go/`.

### 3.1 GET a JSON resource
- **Goal:** one shared client, `NewRequestWithContext`, status check, JSON decode.
- **Run:** `go run ./modules/07-http-client/examples/01_get_json`
- **Code:** [examples/01_get_json/main.go](examples/01_get_json/main.go)
```go
resp, err := client.Do(req)
if err != nil { return err }
defer resp.Body.Close()
if resp.StatusCode < 200 || resp.StatusCode > 299 {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return &StatusError{Code: resp.StatusCode, Body: string(body)}
}
return json.NewDecoder(resp.Body).Decode(out)
```
- **Walkthrough:** `client.Do` returns an error only for transport problems; a `404` arrives as a normal `*http.Response`, so the code converts it to `*StatusError`. The body of an error response is read through `io.LimitReader` to stay bounded.
- **Expected output:**
  ```
  got {ID:1 Title:Write docs Status:todo}
  status 404, body "no such task\n"
  ```

### 3.2 Timeouts and cancellation
- **Goal:** know the three ways a request ends early and how to tell them apart.
- **Run:** `go run ./modules/07-http-client/examples/02_timeouts`
- **Code:** [examples/02_timeouts/main.go](examples/02_timeouts/main.go)
- **Walkthrough:** `Client.Timeout` bounds the whole exchange including reading the body; a context deadline bounds one request; `cancel()` gives `context.Canceled`. `Client.Timeout` errors satisfy both `os.IsTimeout` and `errors.Is(err, context.DeadlineExceeded)`. `Transport` fields (`ResponseHeaderTimeout`, `IdleConnTimeout`) tune individual phases.
- **Expected output:**
  ```
  client timeout: true true
  context deadline: true
  cancelled: true false
  transport configured: 2s
  ```

### 3.3 Connection reuse needs a drained body
- **Goal:** see why `defer resp.Body.Close()` alone is not always enough.
- **Run:** `go run ./modules/07-http-client/examples/03_connection_reuse`
- **Code:** [examples/03_connection_reuse/main.go](examples/03_connection_reuse/main.go)
- **Walkthrough:** the server counts new TCP connections (`ConnState`). Five requests whose bodies are fully read then closed share one connection. If a *large* body is closed unread, the transport cannot reuse the connection and dials again. (Tiny bodies are often buffered already, so the effect only shows reliably above ~1 MiB.)
- **Expected output:**
  ```
  drain and close        -> 1 connection(s) for 5 requests
  close without reading  -> 5 connection(s) for 5 requests
  ```

### 3.4 Retry with exponential backoff
- **Goal:** retry only what is safe to retry.
- **Run:** `go run ./modules/07-http-client/examples/04_retry_backoff`
- **Code:** [examples/04_retry_backoff/main.go](examples/04_retry_backoff/main.go)
```go
if !retryable || !idempotent(method) || attempt == retries {
	return nil, fmt.Errorf("giving up after %d attempt(s): %w", attempt+1, lastErr)
}
select {
case <-time.After(backoff(base, attempt)): // base << attempt
case <-ctx.Done():
	return nil, ctx.Err()
}
```
- **Walkthrough:** network errors and `5xx` are retried; `4xx` never is (the answer would not change); `POST` is never retried because the first attempt may have been processed. A new request is built per attempt with `bytes.NewReader(body)` since a consumed reader cannot be replayed. The wait honours the context. Production code usually adds random jitter.
- **Expected output:**
  ```
  flaky GET    calls=3 status=200
  missing GET  calls=1 status=404
  flaky POST   calls=1 error=giving up after 1 attempt(s): upstream status 503
  schedule (base 100ms): 100ms 200ms 400ms 800ms
  cancelled context: true
  ```

### 3.5 A typed Task Tracker client
- **Goal:** put it together: auth header, query encoding, JSON in/out, error envelope decoded into `*APIError`.
- **Run:** `go run ./modules/07-http-client/examples/05_task_client`
- **Code:** [examples/05_task_client/main.go](examples/05_task_client/main.go)
```go
req.Header.Set("Authorization", "Bearer "+c.token)
...
if resp.StatusCode >= 400 {
	var env struct{ Error APIError `json:"error"` }
	json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&env)
	env.Error.Status = resp.StatusCode
	return &env.Error
}
```
- **Walkthrough:** every method funnels through `do`, so headers, body closing and error mapping exist exactly once. `APIError.Is` lets callers write `errors.Is(err, ErrNotFound)` without knowing status codes. The server is an in-process fake of the API from the shared domain.
- **Expected output:**
  ```
  create: {ID:1 Title:Write docs Status:todo Tags:[docs]} err=<nil>
  create invalid: status=422 code=validation_failed fields=map[title:must be 1-200 characters]
  get 42: 404 not_found: task not found | is not found: true
  list: total=1 limit=10 first="Write docs" err=<nil>
  bad token: 401 unauthorized: missing or invalid credentials
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `http.Get(url)` / `http.DefaultClient` in a service | The default client has **no timeout**; one stuck peer blocks a goroutine forever | `&http.Client{Timeout: ...}` plus a request context |
| Creating a new `http.Client`/`Transport` per call | Looks harmless, but each `Transport` has its own pool: sockets and goroutines leak | One shared client (package variable or struct field) |
| Forgetting `resp.Body.Close()` (also on non-2xx and decode-error paths) | Early `return` before the `defer` is registered, or body ignored on errors | `defer resp.Body.Close()` immediately after the error check |
| Closing without draining | Unread large bodies make the transport drop the connection | `io.Copy(io.Discard, resp.Body)` (bounded) before closing when you ignore a body |
| `err == nil` means success | `Do` returns nil error for `404` and `500` | Check `resp.StatusCode` and map to a typed error |
| `defer resp.Body.Close()` before checking `err` | On error `resp` is nil, so the deferred call panics | Check `err` first, then defer |
| Retrying `POST` or `4xx` | Retry loop only looks at "did it fail" | Retry idempotent methods on network error/`5xx` only; rebuild the request each attempt |
| Reusing an `io.Reader` body in a retry | The reader was consumed by the first attempt | Keep `[]byte` and create a new reader per attempt |
| `time.Sleep` in the backoff loop | It ignores cancellation | `select` on `time.After`/timer and `ctx.Done()` |
| Unbounded `io.ReadAll(resp.Body)` | A hostile or buggy peer can send gigabytes | `io.LimitReader` / `http.MaxBytesReader` equivalents |

## 5. Coming From Other Languages
- **Python `requests` / JS `fetch`:** there is no `raise_for_status` or `res.ok`; you write the status check (3.1). `context.Context` replaces `timeout=` kwargs and `AbortController`, and works for every layer.
- **Java `HttpClient` / C# `HttpClient`:** same advice applies: one long-lived instance. Go has no checked exceptions: transport failures are returned `error`s, HTTP failure statuses are plain data.
- **Rust `reqwest`:** similar split (client vs request), but Go's `Body` is closed by you, not by drop; the `defer` is mandatory.
- Mocking: instead of a mocking library you inject `http.RoundTripper` (or a tiny `Doer` interface) or point the client at `httptest.NewServer`.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/) (same relative paths).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | `GetJSON` helper | Easy | status mapping, body closing, wrapping errors | `go test ./modules/07-http-client/exercises/ex1_get_json/` |
| 2 | `retry.Do` | Medium | idempotency, backoff, rebuilding bodies, context-aware waits | `go test ./modules/07-http-client/exercises/ex2_retry/` |
| 3 | Task Tracker client | Stretch | one `do` helper, query encoding, `APIError`, pagination with `iter.Seq2` | `go test ./modules/07-http-client/exercises/ex3_task_client/` |

**Exercise 1: `GetJSON`**
- **Task:** implement `GetJSON(ctx, client, url, out)` in [exercises/ex1_get_json/getjson.go](exercises/ex1_get_json/getjson.go).
- **Acceptance:** decodes 2xx JSON and sends `Accept`; non-2xx gives `*StatusError` with at most 1024 body bytes; every path closes the body; transport errors stay matchable with `errors.Is`; a cancelled context is honoured.
- <details><summary>Hint</summary>The tests use a fake `RoundTripper`, so nothing is listening: only your use of the `*http.Response` matters. Read the error-body through `io.LimitReader`.</details>

**Exercise 2: `retry.Do`**
- **Task:** implement `Do` in [exercises/ex2_retry/retry.go](exercises/ex2_retry/retry.go) following the rules in its doc comment.
- **Acceptance:** sleeps are `Base, 2*Base, ...` and never follow the last attempt; `POST`/`PATCH`, `4xx` and `3xx` are never retried; retried bodies are closed; the request body is re-sent; exhausted retries return the last result; cancelling the context ends the wait.
- <details><summary>Hint</summary>Decide "may I retry?" first (`retryable && idempotent && attempt < MaxRetries`); only then close the old response and wait.</details>

**Exercise 3: Task Tracker client**
- **Task:** implement `New`, `Create`, `Get`, `SetStatus`, `Delete`, `List`, `All` in [exercises/ex3_task_client/taskclient.go](exercises/ex3_task_client/taskclient.go).
- **Acceptance:** required headers on every request (`Content-Type` only with a body); query values escaped and zero options omitted; error envelope decoded (and a non-JSON error body still yields `*APIError`); `errors.Is(err, ErrNotFound)` for 404; `All` follows pages and stops fetching when the loop breaks.
- <details><summary>Hint</summary>Write one private `do(ctx, method, path, query, in, out)`; make `out == nil` mean "ignore the body" for `204`.</details>

## 7. Recap
- One shared `http.Client` with a `Timeout`; every request built with `NewRequestWithContext`.
- `Do` fails only on transport problems: check the status yourself and turn it into a typed error.
- Always close the body; drain it when you want the connection reused.
- Retry only idempotent requests on network errors/`5xx`, with backoff that honours the context.
- Reach for `httptest.NewServer` to test the wire behaviour and for a fake `RoundTripper` (or small interface) to test logic without sockets.

## 8. Self-check Questions
1. Why does `client.Get("http://x/missing")` return `err == nil` for a 404, and where should the check for it live?
2. What is the difference between `Client.Timeout` and a context deadline, and which one still applies when the caller cancels?
3. Why is it unsafe to retry a `POST` after a network error, even though no response was received?
4. A service creates `&http.Client{}` inside each handler and calls `defer resp.Body.Close()` but never reads the body. What symptoms will it show under load?

## 9. Further Reading
- [`net/http` package docs](https://pkg.go.dev/net/http) (`Client`, `Transport`, `Response.Body`)
- [`net/http/httptest`](https://pkg.go.dev/net/http/httptest)
- Go blog: [Go Concurrency Patterns: Context](https://go.dev/blog/context)
- Cloudflare: [The complete guide to Go net/http timeouts](https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/)

---
**Next:** [08 File and Stream I/O](../08-file-and-stream-io/README.md)
