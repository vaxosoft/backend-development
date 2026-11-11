# 02. HTTP Server in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md)

## Learning Outcomes
After this module you can:
- [ ] Route requests and read path, query, header, JSON, form and multipart data with axum extractors
- [ ] Write responses with status codes, headers, cookies, negotiated content and streamed bodies; serve static files
- [ ] Add middleware (request id, timing, timeouts, body limits, CORS) and predict the order layers run in
- [ ] Share state with handlers safely using `State`
- [ ] Start a server on a configurable address and shut it down gracefully on Ctrl-C/SIGTERM

## 1. Concept Overview
An HTTP server is a loop that accepts connections, parses each request into method, path, headers and body, finds a **handler** through a **router**, and writes the handler's response back. **Middleware** wraps handlers to add cross-cutting behaviour (ids, logging, timeouts) without touching them. **Graceful shutdown** stops accepting new connections but lets in-flight requests finish.

```
client -> [layers: request-id -> cors -> timeout] -> router -> handler(extractors) -> response
                 (response flows back out through the same layers)
```

## 2. How Rust Approaches It
- **axum 0.8** on **tokio** (+ `tower`, `tower-http`) is the de facto stack. A handler is a plain `async fn`; its **parameters are extractors** (`Path`, `Query`, `Json`, `State`, `HeaderMap`...) and its **return type** implements `IntoResponse`. There are no annotations or reflection: the types are the routing contract, checked at compile time.
- `Router::new().route("/tasks/{id}", get(read).delete(remove))` (axum 0.8 path syntax is `{id}`, wildcard `{*rest}`); `.fallback(..)` handles unknown paths; `.layer(..)` wraps everything added before it.
- Middleware is either an async fn via `middleware::from_fn` or a `tower::Layer` (`TimeoutLayer`, `CorsLayer`, `DefaultBodyLimit`...). Layers compose; the **last** `.layer()` call is the **outermost**.
- Shared state is `State<T>` where `T: Clone` (usually an `Arc`). Only `with_state(..)` makes it available.
- `axum::serve(listener, app).with_graceful_shutdown(signal)` drains connections. Per-request deadlines come from `TimeoutLayer`; connection-level header timeouts need `hyper-util` and are out of scope here.
- Avoid: blocking calls inside handlers (use `tokio::fs`, `spawn_blocking`), `unwrap()` on request data, a body extractor that is not the last parameter.

Small shared helpers live in [src/lib.rs](src/lib.rs): `bind_addr()` (reads `ADDR`, default `127.0.0.1:8080`) and `shutdown_signal()` (Ctrl-C or SIGTERM).

## 3. Guided Examples
All commands run from `languages/rust/`. Servers read `ADDR` (e.g. `ADDR=127.0.0.1:9000`) and stop on Ctrl-C. Use a second terminal for the `curl` commands. Outputs omit the `date` header.

### 3.1 Routing, path and query parameters, shutdown
- **Goal:** the shape of an axum app.
- **Run:** `cargo run -p m02-http-server --example hello`
- **Code:** [examples/hello.rs](examples/hello.rs)
```rust
fn app() -> Router {
    Router::new()
        .route("/hello/{name}", get(hello))
        .route("/users/{user}/tasks/{task}", get(user_task))
        .route("/assets/{*rest}", get(static_like))
        .fallback(not_found)
}
async fn hello(Path(name): Path<String>, Query(q): Query<Greeting>) -> String { /* ... */ }
```
- **Walkthrough:** `Path<(u32, u32)>` destructures two segments and parses them; a bad value is rejected with `400` before the handler runs. `Greeting` marks `greeting` as optional with `#[serde(default = ..)]`. `main` binds a `TcpListener`, then `serve(..).with_graceful_shutdown(shutdown_signal())`: on SIGTERM the process prints a message and exits after in-flight requests finish.
- **Expected output:**
  ```
  $ curl -si localhost:8080/hello/Ada
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  content-length: 12

  Hello, Ada!
  $ curl -s 'localhost:8080/hello/Ada?greeting=Hi'
  Hi, Ada!
  $ curl -s localhost:8080/users/7/tasks/42
  user 7, task 42
  $ curl -si localhost:8080/users/x/tasks/1
  HTTP/1.1 400 Bad Request
  content-type: text/plain; charset=utf-8
  content-length: 68

  Invalid URL: Cannot parse value at index 0 with value `x` to a `u32`
  $ curl -s localhost:8080/assets/css/site.css
  wildcard captured 'css/site.css'
  $ curl -si localhost:8080/nope
  HTTP/1.1 404 Not Found
  content-type: text/plain; charset=utf-8
  content-length: 13

  nothing here
  ```

### 3.2 Extractors: `State`, `Path`, `Query`, `Json`, `Form`, `Bytes`, `Multipart`
- **Goal:** read every part of a request; learn the extractor-order rule.
- **Run:** `cargo run -p m02-http-server --example extractors`
- **Code:** [examples/extractors.rs](examples/extractors.rs)
```rust
// State and Path read only the request head; Json consumes the body, so it is LAST.
async fn replace_task(
    State(state): State<AppState>,
    Path(id): Path<u32>,
    Json(input): Json<TaskInput>,
) -> Json<serde_json::Value> { /* ... */ }
```
- **Walkthrough:** `AppState` holds an `Arc<AtomicU64>`; cloning it per request is cheap and all clones share the counter. `Query<Search>` makes `q` required and `limit` optional. `Multipart` yields fields one by one (`field.bytes()` buffers; module 08 streams to disk). Rejections (bad JSON, wrong content type, missing field) are produced by the extractor, before your code.
- **Expected output:**
  ```
  $ curl -s localhost:8080/counter; curl -s localhost:8080/counter
  hit 1
  hit 2
  $ curl -s 'localhost:8080/search?q=rust&tags=a,b&limit=5'
  q=rust tags=["a", "b"] limit=5
  $ curl -s 'localhost:8080/search?tags=a'
  Failed to deserialize query string: missing field `q`
  $ curl -s -X PUT -H 'Content-Type: application/json' -d '{"title":"  Write docs  "}' localhost:8080/tasks/3
  {"id":3,"title":"Write docs"}
  $ curl -si -X PUT -d '{"title":"x"}' localhost:8080/tasks/3 | head -1
  HTTP/1.1 415 Unsupported Media Type
  $ curl -si -X PUT -H 'Content-Type: application/json' -d '{"name":"x"}' localhost:8080/tasks/3 | head -1
  HTTP/1.1 422 Unprocessable Entity
  $ curl -s -d 'email=ada%40example.com' localhost:8080/login
  form email: ada@example.com
  $ curl -s -F title=Report -F file=@up.txt localhost:8080/upload      # up.txt contains "abc"
  title file=None 6 bytes
  file file=Some("up.txt") 3 bytes
  $ curl -si -X DELETE localhost:8080/items/1 | head -2
  HTTP/1.1 405 Method Not Allowed
  allow: GET,HEAD
  ```

### 3.3 Responses: status, headers, negotiation, cookies, streaming, static files
- **Goal:** everything a handler can return.
- **Run:** `cargo run -p m02-http-server --example responses`
- **Code:** [examples/responses.rs](examples/responses.rs), static file [examples/static/hello.txt](examples/static/hello.txt)
```rust
async fn created() -> impl IntoResponse {
    (StatusCode::CREATED, [(header::LOCATION, "/tasks/1")], Json(json!({ "id": 1 })))
}
async fn set_theme(jar: CookieJar) -> (CookieJar, &'static str) { (jar.add(cookie), "theme saved\n") }
Body::from_stream(chunks)   // no Content-Length, chunked transfer encoding
```
- **Walkthrough:** tuples of `(StatusCode, headers, body)` implement `IntoResponse`. `negotiate` inspects `Accept` and answers `406` for unsupported types. `CookieJar` (from `axum-extra`) is both an extractor and a response part. `countdown` sends three chunks 20 ms apart. `ServeDir` (tower-http) serves files with correct `content-type`, `last-modified`, `etag` and range support.
- **Expected output:**
  ```
  $ curl -si localhost:8080/created
  HTTP/1.1 201 Created
  content-type: application/json
  location: /tasks/1
  content-length: 29

  {"id":1,"title":"Write docs"}
  $ curl -si localhost:8080/old-path | head -2
  HTTP/1.1 308 Permanent Redirect
  location: /new-path
  $ curl -s -H 'Accept: text/plain' localhost:8080/negotiate
  hello
  $ curl -si -H 'Accept: text/csv' localhost:8080/negotiate | head -1
  HTTP/1.1 406 Not Acceptable
  $ curl -si -X POST localhost:8080/theme | grep -i set-cookie
  set-cookie: theme=dark; HttpOnly; Path=/
  $ curl -s -H 'Cookie: theme=dark' localhost:8080/theme
  theme: dark
  $ curl -si --raw localhost:8080/countdown
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  transfer-encoding: chunked

  7
  tick 1

  7
  tick 2

  7
  tick 3

  0

  $ curl -si localhost:8080/static/hello.txt | grep -i '^content-type\|^accept-ranges'
  content-type: text/plain
  accept-ranges: bytes
  $ curl -s localhost:8080/static/hello.txt
  Static files are served by tower-http's ServeDir.
  ```

### 3.4 Middleware: `from_fn`, layer order, timeout, body limit, CORS
- **Goal:** cross-cutting behaviour without touching handlers.
- **Run:** `cargo run -p m02-http-server --example middleware`
- **Code:** [examples/middleware.rs](examples/middleware.rs)
```rust
Router::new()
    .route("/order", get(order))
    .route("/upload", post(upload).layer(DefaultBodyLimit::max(16)))
    .layer(middleware::from_fn(|req, next| tag("inner", req, next)))
    .layer(middleware::from_fn(|req, next| tag("outer", req, next)))   // added later = outermost
    .layer(TimeoutLayer::with_status_code(StatusCode::REQUEST_TIMEOUT, Duration::from_millis(200)))
    .layer(cors)
    .layer(middleware::from_fn_with_state(Ids::default(), request_id))
```
- **Walkthrough:** `request_id` echoes an incoming `x-request-id` or generates `req-N`, stores it on the request for handlers and copies it onto the response; it is the outermost layer, so it also decorates timeouts and CORS answers. `tag` shows the order: the handler sees `outer,inner`. `/slow?ms=1000` exceeds the 200 ms timeout and gets `408`. CORS is enforced by browsers, not curl: the server only announces what is allowed (the preflight answer lists methods and headers).
- **Expected output:**
  ```
  $ curl -si localhost:8080/order
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  access-control-allow-origin: https://app.example
  x-request-id: req-1
  content-length: 12

  outer,inner
  $ curl -si -H 'X-Request-ID: abc-123' localhost:8080/order | grep -i x-request-id
  x-request-id: abc-123
  $ curl -si 'localhost:8080/slow?ms=1000'
  HTTP/1.1 408 Request Timeout
  access-control-allow-origin: https://app.example
  x-request-id: req-2
  content-length: 0

  $ curl -si -d 0123456789abcdefghij localhost:8080/upload | head -1
  HTTP/1.1 413 Payload Too Large
  $ curl -si -X OPTIONS -H 'Origin: https://app.example' -H 'Access-Control-Request-Method: POST' \
      -H 'Access-Control-Request-Headers: content-type' localhost:8080/upload
  HTTP/1.1 200 OK
  access-control-allow-methods: GET,POST
  access-control-allow-headers: content-type
  access-control-allow-origin: https://app.example
  x-request-id: req-4
  allow: POST
  content-length: 0

  ```
  (Ids count generated ones only: `req-3` went to the `413` request; the echoed `abc-123` did not consume a number.)

### 3.5 Task Tracker server
- **Goal:** the pieces together in the shared domain: state behind a lock, JSON in and out, `201 + Location`, `404`, `204`, a timeout layer and graceful shutdown.
- **Run:** `cargo run -p m02-http-server --example tasks_server`
- **Code:** [examples/tasks_server.rs](examples/tasks_server.rs)
```rust
type Db = Arc<Mutex<Store>>;   // std Mutex: lock, copy what you need, release BEFORE any .await
async fn get_one(State(db): State<Db>, Path(id): Path<u32>) -> Result<Json<Task>, StatusCode> {
    let store = db.lock().expect("store mutex poisoned");
    store.tasks.get(&id).cloned().map(Json).ok_or(StatusCode::NOT_FOUND)
}
```
- **Walkthrough:** `Result<Json<Task>, StatusCode>` is a valid return type because both sides implement `IntoResponse`. The `created_at` value is a fixed stand-in so the output is reproducible; real clocks arrive in module 13. Error bodies are empty here; module 03 introduces a proper error envelope.
- **Expected output:**
  ```
  $ curl -s -H 'Content-Type: application/json' -d '{"title":"  Write docs "}' -i localhost:8080/tasks
  HTTP/1.1 201 Created
  content-type: application/json
  location: /tasks/1
  content-length: 81

  {"id":1,"title":"Write docs","status":"todo","created_at":"2030-01-01T09:00:00Z"}
  $ curl -s localhost:8080/tasks/1
  {"id":1,"title":"Write docs","status":"todo","created_at":"2030-01-01T09:00:00Z"}
  $ curl -si localhost:8080/tasks/9 | head -1
  HTTP/1.1 404 Not Found
  $ curl -si -X DELETE localhost:8080/tasks/1 | head -1
  HTTP/1.1 204 No Content
  $ curl -s localhost:8080/tasks
  []
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Body extractor (`Json`, `Form`, `Bytes`, `Multipart`) before another extractor | the body can be consumed once; axum requires the last parameter to take it | put exactly one body extractor last |
| Handler fails to compile with a huge `Handler<_, _>` error | a parameter is not an extractor, the return type is not `IntoResponse`, or the future is not `Send` | add `#[axum::debug_handler]` (macros feature) for a readable error |
| `std::sync::MutexGuard` held across `.await` | the guard is not `Send` | scope the lock in a block and drop it before awaiting, or use `tokio::sync::Mutex` when you must hold it |
| `State<T>` requires `Clone`; forgetting `.with_state(..)` | state is per-router, cloned per request | wrap in `Arc`; call `with_state` after the routes |
| Old path syntax `/:id` or `/*rest` | axum 0.7 and earlier | axum 0.8 uses `/{id}` and `/{*rest}` |
| `.layer()` ordering surprises | each `.layer` wraps all routes added so far, and the last one is outermost | add routes first, then layers; check order with a header like `/order` |
| `Query<Vec<String>>` for `?tag=a&tag=b` fails | `serde_urlencoded` has no repeated-key lists | accept `tag=a,b` and split, or parse the raw query |
| Blocking work (`std::thread::sleep`, big file reads) in a handler | a tokio worker thread stalls all tasks on it | `tokio::time::sleep`, `tokio::fs`, `spawn_blocking` |
| Default extractor errors are plain text | rejections come from axum | wrap them in your own extractor (module 03) |
| Server never exits on SIGTERM | only Ctrl-C was handled | `shutdown_signal()` waits for both |

## 5. Coming From Other Languages
- **Go `net/http` + chi:** handlers are `func(w, r)`; here the parameters *are* the parsed request, so there is no `r.PathValue`/`json.NewDecoder` boilerplate. Middleware is `tower` layers instead of `func(http.Handler) http.Handler`.
- **Express/Koa:** `from_fn` middleware looks similar (`next.run(req)` = `await next()`), but order is inside-out with `.layer`.
- **Spring/ASP.NET:** no DI container or attributes; "dependency injection" is `State` plus constructor functions.
- **C#/Java:** model binding errors become *rejections*; the `IntoResponse` trait plays the role of `IActionResult`.

## 6. Exercises

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Greeter router](exercises/ex1-greeter) | Easy | routes, `Path`, `Query`, `Json` response, fallback | `cargo test -p m02-ex1-greeter` |
| 2 | [Request id + counters](exercises/ex2-request-id) | Medium | `State`, `from_fn` middleware, header mutation | `cargo test -p m02-ex2-request-id` |
| 3 | [Graceful shutdown](exercises/ex3-graceful-shutdown) | Stretch | `with_graceful_shutdown`, grace timer, `select!` | `cargo test -p m02-ex3-graceful-shutdown` |

**1. Greeter router.** *Task:* fill in the five handlers in `router()` as described in the crate docs. *Acceptance:* all tests in `tests/greeter.rs` (they call the router in-process with `tower::ServiceExt::oneshot`, no sockets). *Hint:* <details><summary>nudge</summary>`Option<String>` in the query struct makes the parameter optional; `Path<(u32, u32)>` destructures two segments.</details>

**2. Request id + counters.** *Task:* implement the `request_id` middleware, `GET /whoami`, and the counter endpoints. *Acceptance:* `tests/request_id.rs`: generated ids are sequential per router, an incoming id is echoed without consuming the counter, error responses carry the header too. *Hint:* <details><summary>nudge</summary>`req.headers_mut().insert(..)` before `next.run(req).await`, then insert into `response.headers_mut()`.</details>

**3. Graceful shutdown.** *Task:* implement `serve(listener, app, shutdown, grace)`. In-flight requests finish, new connections are refused, but after `grace` it returns `Outcome::TimedOut` even if a handler is stuck. *Acceptance:* `tests/shutdown.rs` (real loopback sockets, port 0, no sleeps longer than 50 ms). *Hint:* <details><summary>nudge</summary>`.with_graceful_shutdown(..)` returns a builder: call `.into_future()` to get a future you can put in `tokio::select!` next to a timer that starts when the shutdown signal fires (send on a `oneshot` from inside the shutdown future).</details>

## 7. Recap
- A handler is an `async fn`; extractors in, `IntoResponse` out; the body extractor goes last.
- `Router` + `.layer()` composes middleware; the last layer added runs first.
- `State` needs `Clone`: wrap shared data in `Arc` and never hold a std mutex guard across `.await`.
- `with_graceful_shutdown` drains in-flight requests; add your own grace deadline for stuck handlers.
- Reach for `from_fn` for small custom middleware, tower-http layers for the standard ones (timeout, CORS, static files).

## 8. Self-check Questions
1. Why must `Json<T>` be the last parameter, but `State<T>` and `Path<T>` can be anywhere?
2. With `.layer(A).layer(B)`, which runs first on the request and which first on the response?
3. Why is `Arc<Mutex<..>>` cloned per request without copying the data?
4. What does graceful shutdown guarantee, and what does it not (hint: a request that never finishes)?
5. Why can curl not "violate" CORS, and what does the server actually do?

## 9. Further Reading
- [axum docs](https://docs.rs/axum/0.8) (especially "Extractors", "Handlers" and "Middleware")
- [tower-http](https://docs.rs/tower-http) and the [axum examples](https://github.com/tokio-rs/axum/tree/main/examples) (graceful-shutdown, cors, multipart-form)
- [MDN: HTTP overview and status codes](https://developer.mozilla.org/en-US/docs/Web/HTTP)

---
**Next:** [03 REST API Design](../03-rest-api-design/README.md)
