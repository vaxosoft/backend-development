# 07. HTTP Client in Python

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01 (language fundamentals), 04 (serialization), 06 (concurrency, for the async parts)

## Learning Outcomes
After this module you can:
- [ ] Send requests with query parameters, headers and JSON or form bodies, and read status, headers and the decoded body
- [ ] Reuse one `httpx.Client` and configure connect/read/write/pool timeouts
- [ ] Treat non-2xx responses and transport failures as different things and handle both
- [ ] Retry idempotent requests with exponential backoff and jitter, and never retry 4xx
- [ ] Stream large bodies and run bounded-concurrency calls with `AsyncClient`
- [ ] Test client code without a network using `httpx.MockTransport`

## 1. Concept Overview
An HTTP client is the mirror image of the server you built in modules 02-03: it builds a request (method, URL, headers, body), waits for a response and turns bytes back into data. Three things make production clients different from a one-line `GET`: they **reuse connections** (TCP and TLS setup is expensive), they **bound every wait** (a hung upstream must not hang you) and they **separate failure kinds**: the server answered with an error status, or no answer arrived at all.

Retries are the usual reaction to the second kind of failure, but only when they are safe: the request must be idempotent, the delay must grow (exponential backoff) with randomness (jitter) so that many clients do not retry in lockstep, and the total number of attempts must be small. Authentication is just another header (`Authorization: Bearer ...`) that you attach in one place.

## 2. How Python Approaches It

| Tool | Use it for |
|------|-----------|
| `httpx.Client` / `httpx.AsyncClient` | the reusable client: connection pool, base URL, default headers, auth, timeouts |
| `httpx.Timeout(5.0, connect=1.0)` | separate connect/read/write/pool limits (each one is per operation, not a total deadline) |
| `response.raise_for_status()` | turns 4xx/5xx into `httpx.HTTPStatusError` when you want an exception |
| `httpx.HTTPError` > `RequestError` > `TransportError` > `TimeoutException`, `NetworkError` | the exception tree to catch narrowly |
| `client.stream(...)` | iterate a body (`iter_bytes`, `iter_lines`) without loading it |
| `httpx.MockTransport(handler)` | a fake network: your function receives each `httpx.Request` and returns a `httpx.Response` |
| `httpx.Auth` subclass | attach credentials to every request in one place |

The sync API is the right default for scripts and plain `def` handlers. `AsyncClient` has the same API with `await`, and is what you want inside `async def` code or when fanning out many calls. The stdlib has `urllib.request`, which is fine for a single script line but has no pooling, no timeouts by default and a clumsy API ([PROFILE](../../PROFILE.md) picks `httpx`). Retries are hand-written here (about 20 lines) so you see exactly what they do; `httpx.HTTPTransport(retries=n)` only retries *connection* failures.

## 3. Guided Examples
All examples start a throw-away loopback server (`examples/m07_helpers.py`, stdlib `http.server`) so they run offline. Run them from `languages/python/`.

### 3.1 First requests
- **Goal:** query parameters, headers and the three body styles.
- **Run:** `python modules/07-http-client/examples/01_first_requests.py`
- **Code:** [01_first_requests.py](examples/01_first_requests.py)

```python
response = httpx.get(f"{base_url}/echo", params={"name": "Ada Lovelace", "tag": ["a", "b"]})
response = httpx.post(f"{base_url}/echo", json={"title": "Write docs", "tags": ["docs"]})
response = httpx.post(f"{base_url}/echo", data={"title": "Write docs"})  # form body
response = httpx.put(f"{base_url}/echo", content=b"raw bytes")
```

- **Walkthrough:** `params=` encodes the query string (a list repeats the key). `json=` serialises and sets `Content-Type`. `data=` sends `application/x-www-form-urlencoded`, `content=` sends bytes untouched.
- **Expected output:**

  ```
  200 application/json
  {'method': 'GET', 'path': '/echo', 'query': 'name=Ada+Lovelace&tag=a&tag=b', 'accept': 'application/json', 'content_type': None, 'body': ''}
  application/json {'title': 'Write docs', 'tags': ['docs']}
  application/x-www-form-urlencoded title=Write+docs
  PUT raw bytes
  ```

### 3.2 One Client, timeouts and errors
- **Goal:** the difference between an error *response* and an error *transport*, and why the client is reused.
- **Run:** `python modules/07-http-client/examples/02_client_timeouts_errors.py`
- **Code:** [02_client_timeouts_errors.py](examples/02_client_timeouts_errors.py)

```python
with httpx.Client(base_url=base_url, timeout=httpx.Timeout(2.0, connect=0.5)) as client:
    response = client.get("/tasks/42")  # 404 is a normal response
    response.raise_for_status()  # ... until you ask for an exception
    client.get("/slow", timeout=0.05)  # httpx.ReadTimeout
```

- **Walkthrough:** `HTTPStatusError` carries the `response`; `ReadTimeout` and `ConnectError` are `TransportError`s with no response at all. The last block counts the client ports the server saw: one `Client` keeps one keep-alive connection, three bare `httpx.get` calls open three.
- **Expected output:**

  ```
  status: 404 success: False
  raised: HTTPStatusError not_found
  raised: ReadTimeout True
  raised: ConnectError True
  one Client, 3 requests, connections: 1
  httpx.get x3, connections: 3
  ```

### 3.3 Retry with backoff and jitter
- **Goal:** retry only what is safe, with growing random delays.
- **Run:** `python modules/07-http-client/examples/03_retry_backoff.py`
- **Code:** [03_retry_backoff.py](examples/03_retry_backoff.py)

```python
attempts = retries + 1 if request.method in IDEMPOTENT else 1
for attempt in range(attempts):
    ...  # TransportError or 429/502/503/504 -> try again unless this was the last attempt
    sleep(rng.uniform(0, base_delay * 2**attempt))  # "full jitter"
```

- **Walkthrough:** `sleep` and `rng` are parameters, so the example (and your tests) never really wait and the jitter is reproducible. The delay ceiling doubles per attempt (0.1, 0.2, 0.4 s). A 404 returns at once; a POST is sent once even though the server says 503.
- **Expected output:**

  ```
  GET /flaky: 200 after 3 call(s), waits=2
  GET /bad: 404 after 1 call(s), waits=0
  GET /down: 503 after 4 call(s), waits=3
  POST /down: 503 after 1 call(s), waits=0
  ceilings: [0.1, 0.2, 0.4]
  ```

### 3.4 Streaming and async fan-out
- **Goal:** read a body piece by piece, and run several calls concurrently with a cap.
- **Run:** `python modules/07-http-client/examples/04_streaming_and_async.py`
- **Code:** [04_streaming_and_async.py](examples/04_streaming_and_async.py)

```python
with client.stream("GET", "/big") as response:
    for chunk in response.iter_bytes(chunk_size=64 * 1024):
        digest.update(chunk)

limit = asyncio.Semaphore(2)
async with limit:
    response = await client.get(f"/items/{item_id}")
```

- **Walkthrough:** `stream()` returns before the body is read, so memory stays flat. In the async part six requests share one `AsyncClient`; the semaphore keeps at most two in flight (the server records the peak) and `TaskGroup` keeps results in request order.
- **Expected output:**

  ```
  ['{"n": 0}', '{"n": 1}', '{"n": 2}', '{"n": 3}', '{"n": 4}']
  downloaded 1048576 bytes, sha256 matches: True
  results in request order: [1, 2, 3, 4, 5, 6]
  peak concurrent requests at the server: 2
  ```

### 3.5 A Task Tracker API client (tested with `MockTransport`)
- **Goal:** wrap the API in a typed class, parse the shared error envelope and test it with no network.
- **Run:** `python modules/07-http-client/examples/05_task_tracker_client.py`
- **Code:** [05_task_tracker_client.py](examples/05_task_tracker_client.py)

```python
class BearerAuth(httpx.Auth):
    def auth_flow(self, request):
        request.headers["Authorization"] = f"Bearer {self._token}"
        yield request


transport = httpx.MockTransport(FakeTaskTracker())  # the "server" is a callable
httpx.Client(transport=transport, base_url="http://tracker.test", auth=BearerAuth("s3cret"))
```

- **Walkthrough:** `TaskTrackerClient` receives the `httpx.Client` (dependency injection), converts JSON into a frozen `Task`, turns `{"error": {...}}` bodies into `ApiError` and pages through `/tasks` with `limit`/`offset`. The fake records which `Authorization` headers it saw.
- **Expected output:**

  ```
  Task(id=1, title='Write docs', status='todo', tags=['docs'])
  ['Write docs', 'Ship it', 'Celebrate']
  ApiError: 404 not_found: task not found | code = not_found
  ApiError: 422 validation_failed: invalid task | code = validation_failed
  ApiError: 401 unauthorized: missing or invalid token
  Authorization headers seen by the fake: ['Bearer s3cret', 'None']
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `httpx.get(...)` in a loop or per request | the module-level helpers create and close a client each call: no connection reuse | create one `Client` (or `AsyncClient`) and share it; close it with `with` |
| `timeout=None` | "I don't want it to fail" | keep a finite `httpx.Timeout`; the default 5 s applies per phase (connect, read, ...), it is not a total deadline, so wrap long flows in `asyncio.timeout()` too |
| Assuming `get()` raises on 404/500 | `requests`-style habits; httpx returns the response | call `raise_for_status()` or check `response.is_success` |
| Catching `Exception` around the call | hides programming errors | catch `httpx.HTTPStatusError` and `httpx.TransportError` (or `httpx.HTTPError` for both) separately |
| Retrying `POST` or any 4xx | blanket "retry on error" wrapper | retry only idempotent methods on transport errors and 429/502/503/504; add an idempotency key for POST |
| Fixed `time.sleep(1)` between retries | simple to write | exponential backoff plus jitter; make `sleep` injectable so tests do not wait |
| `response.json()` on a streamed or error body | the body may be HTML or empty | check status and `Content-Type` first, wrap `ValueError` |
| Reading a huge download with `.content` | convenient | `client.stream()` and `iter_bytes()`; remember `response.read()` loads it all |
| Blocking `httpx.Client` inside `async def` | one client for both worlds | use `AsyncClient` with `await` in coroutines (see module 06) |
| Forgetting to close the client | no `with` | use `with`/`async with`, or call `close()`/`aclose()` |

## 5. Coming From Other Languages
- **JavaScript/TypeScript:** `fetch` rejects only on network errors, exactly like `httpx`; `response.raise_for_status()` is the `if (!res.ok) throw` you write by hand. `AbortController` timeouts become `httpx.Timeout`.
- **Java/C#:** `httpx.Client` is `HttpClient`: create once, share, dispose. `MockTransport` replaces the `HttpMessageHandler` you would stub.
- **Go:** `http.Client{Timeout: ...}` is one total timeout; httpx has four per-phase ones. Errors are exceptions instead of returned values, but the "error status is not an error" rule is the same.
- **Ruby:** the `Faraday` middleware stack maps to `httpx.Auth`, event hooks and custom transports.

## 6. Exercises
Starters and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/) (same relative paths). All tests are offline (`MockTransport`).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [JSON helper](exercises/ex1_json_client/m07_json_client.py) | Easy | headers, JSON, error envelope to exception | `pytest modules/07-http-client/exercises/ex1_json_client` |
| 2 | [Retry with backoff](exercises/ex2_retry/m07_retry.py) | Medium | what to retry, backoff, injectable sleep | `pytest modules/07-http-client/exercises/ex2_retry` |
| 3 | [Bounded fan-out](exercises/ex3_fetch_many/m07_fetch_many.py) | Stretch | `AsyncClient`, semaphore, ordered partial results | `pytest modules/07-http-client/exercises/ex3_fetch_many` |

**Exercise 1: `request_json`.** *Task:* send a request, add a bearer token when given, return decoded JSON (or `None` for an empty body) and raise `ApiError(status, code, message)` from the error envelope. *Acceptance:* all tests in `test_m07_json_client.py`, including the non-envelope 502 and an unwrapped `ConnectError`.
<details><summary>Hint</summary>Pass `headers=` and `params=` straight to `client.request`. For the envelope use `response.json()["error"]` inside a `try` that catches `ValueError`, `KeyError` and `TypeError`.</details>

**Exercise 2: `request_with_retry`.** *Task:* retry transport errors and 5xx with delays `base_delay * 2**n`, never retry 4xx or non-idempotent methods, and raise the last failure when attempts run out. *Acceptance:* all tests in `test_m07_retry.py` (delays are recorded by an injected `sleep`).
<details><summary>Hint</summary>Compute `attempts` first, then loop; remember that the last attempt must not sleep.</details>

**Exercise 3: `fetch_many`.** *Task:* `GET /items/{id}` for many ids with at most `max_concurrency` in flight; keep request order and report failures per id as `ItemError` instead of failing everything. *Acceptance:* all tests in `test_m07_fetch_many.py` (the peak concurrency must be exactly the limit).
<details><summary>Hint</summary>One small coroutine per id that acquires an `asyncio.Semaphore`, plus a `TaskGroup`. Catch `httpx.TimeoutException` before `httpx.TransportError`.</details>

## 7. Recap
- Create one client, give it a finite `Timeout`, close it with `with`.
- A response with an error status and a transport failure are different outcomes; handle them separately.
- Retry only idempotent requests, on transient failures, with exponential backoff, jitter and a small cap.
- Stream big bodies; cap fan-out with a semaphore; keep results in order.
- Inject the transport (`MockTransport`) and the `sleep` so client code is testable and fast.

## 8. Self-check Questions
1. Why does `timeout=5` not guarantee that a call returns within 5 seconds?
2. Which of these may be retried automatically and why: `GET /tasks`, `POST /tasks`, `PUT /tasks/1`, `DELETE /tasks/1`?
3. What does jitter protect against, and what would happen to a recovering server without it?
4. How would you test that your client sends the `Authorization` header without starting a server?

## 9. Further Reading
- [httpx documentation](https://www.python-httpx.org/): clients, timeouts, transports, async
- [httpx: Timeouts](https://www.python-httpx.org/advanced/timeouts/) and [Transports](https://www.python-httpx.org/advanced/transports/) (`MockTransport`)
- AWS Architecture Blog, "Exponential Backoff And Jitter"

---
**Next:** [08 File and Stream I/O](../08-file-and-stream-io/README.md)
