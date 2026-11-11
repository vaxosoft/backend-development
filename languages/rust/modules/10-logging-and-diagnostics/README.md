# 10. Logging and Diagnostics in Rust

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 06

## Learning Outcomes
After this module you can:
- [ ] Emit structured events with levels and typed fields using `tracing`, and filter them with `RUST_LOG`-style directives
- [ ] Use spans (`#[instrument]`, `.instrument()`) to attach context such as a request id to every line
- [ ] Write request-logging middleware with correlation ids, and use tower-http's `TraceLayer`
- [ ] Turn handler panics into a 500 error envelope without losing the log line or the server
- [ ] Expose a health endpoint and simple metrics with atomics, and know the first debugging/profiling tools to reach for

## 1. Concept Overview
Logs answer "what happened to *this* request?" at 3 a.m. To be useful they must be **structured** (key/value fields a machine can filter: `task_id=42`), **levelled** (error, warn, info, debug, trace, so you can turn detail up without redeploying), and **correlated** (every line of one request shares a request id). Beyond logs, a service needs a **health endpoint** (is it alive?), a few **metrics** (how many requests, how many failures) and a plan for **crashes** (a panic in one request must not take the process down).

```
event: info!(task_id = 42, "task created")      -> "what happened"
span:  request{request_id=req-0001 method=GET}  -> "in which context" (events inside inherit its fields)
filter: RUST_LOG=info,tasks=debug               -> "how much do I see"
```

## 2. How Rust Approaches It
| Piece | Role |
|-------|------|
| `tracing` | the facade: `info!`/`warn!`/... events, spans, `#[instrument]`, `Instrument` |
| `tracing-subscriber` | the backend: `fmt` (text or `.json()`), `EnvFilter` (`RUST_LOG` directives), `MakeWriter` (where bytes go) |
| `tower-http` | `TraceLayer` (span + completion event per request), `SetRequestIdLayer`/`PropagateRequestIdLayer`, `CatchPanicLayer` |
| `AtomicU64` | lock-free counters for a hand-written metrics endpoint |

- **Idiomatic:** libraries emit events and never install a subscriber; only `main` does, once. Fields are typed (`task_id = 42` is a number in JSON), not interpolated into the message. Spans carry context; async code uses `.instrument(span)` or `#[instrument]`.
- **Avoid:** `println!`/`eprintln!` in servers (no level, no structure), holding `span.enter()` across `.await`, logging whole structs that may hold secrets.
- Timestamps are switched off in the examples (`.without_time()`) **only** so the printed output is stable; keep them in production.
- Profiling/debugging basics (no extra crates): `RUST_BACKTRACE=1` for panics, `dbg!()` for quick looks, `RUST_LOG=debug` at runtime, `cargo build --release` before measuring, `perf`/`cargo flamegraph` for CPU hot spots, `tokio-console` for stuck async tasks, `criterion` benchmarks (module 12).

## 3. Guided Examples
Servers (3.4, 3.5) listen on `ADDR` (default `127.0.0.1:8080`) and stop cleanly on Ctrl-C or SIGTERM.

### 3.1 Events, levels and filter directives
- **Goal:** structured events and what a filter lets through.
- **Run:** `cargo run -p m10-logging-and-diagnostics --example 01_events_and_levels`
- **Code:** [examples/01_events_and_levels.rs](examples/01_events_and_levels.rs)
- **Walkthrough:** `info!(task_id = task.id, status = "todo", "task created")` records fields; `?x` uses `Debug`, `%x` uses `Display`. `target: "tasks::store"` groups events by subsystem. The same `emit()` runs under three `EnvFilter` directive strings: `info`, `debug` and `warn,tasks=trace` (everything at warn, but the `tasks` targets at trace). In a real program you build the filter from the environment: `EnvFilter::try_from_default_env()`, e.g. `RUST_LOG=info,tasks=debug`.
- **Expected output:**
  ```
  --- RUST_LOG=info
   INFO 01_events_and_levels: task created task_id=42 status="todo"
   INFO 01_events_and_levels: full record task=Task { id: 42, title: "Write docs" } title=Write docs
   WARN tasks::store: slow query retries=2
  ERROR tasks::store: query failed error=connection reset
  --- RUST_LOG=debug
  DEBUG tasks::store: executing query sql="SELECT * FROM tasks WHERE id = ?"
  DEBUG 01_events_and_levels: loading task task_id=42
   INFO 01_events_and_levels: task created task_id=42 status="todo"
   INFO 01_events_and_levels: full record task=Task { id: 42, title: "Write docs" } title=Write docs
   WARN tasks::store: slow query retries=2
  ERROR tasks::store: query failed error=connection reset
  --- RUST_LOG=warn,tasks=trace
  DEBUG tasks::store: executing query sql="SELECT * FROM tasks WHERE id = ?"
   WARN tasks::store: slow query retries=2
  ERROR tasks::store: query failed error=connection reset
  ```

### 3.2 Spans and `#[instrument]`
- **Goal:** context that follows the code, including across `.await` and `spawn`.
- **Run:** `cargo run -p m10-logging-and-diagnostics --example 02_spans_and_instrument`
- **Code:** [examples/02_spans_and_instrument.rs](examples/02_spans_and_instrument.rs)
- **Walkthrough:** `#[instrument(skip(title), fields(title_len = title.len()), err)]` opens a span named after the function, records the arguments (except skipped ones), adds a computed field, and logs the `Err` value at ERROR. Spans nest (`create_task{..}:validate{..}`). A field declared `tracing::field::Empty` can be filled later with `Span::current().record(..)`. A task spawned **without** `.instrument(span)` runs outside the span.
- **Expected output:**
  ```
  --- nested spans, and `err` logging the Err value
   INFO create_task{user_id=1 title="Write docs"}:validate{user_id=1 title_len=10}: title is valid
   INFO create_task{user_id=1 title="Write docs"}: storing task
  ERROR create_task{user_id=2 title="  "}:validate{user_id=2 title_len=2}: error=title is empty
  --- async fn with a late-recorded field
   INFO handle{user_id=3 task_id=7}: handling request
  --- spawned task: with and without the span
   INFO job{job_id=9}: inside the span
   INFO span is lost
  ```

### 3.3 JSON logs and capturing output
- **Goal:** machine-readable logs, and the in-memory capture technique used by tests.
- **Run:** `cargo run -p m10-logging-and-diagnostics --example 03_json_logs`
- **Code:** [examples/03_json_logs.rs](examples/03_json_logs.rs)
- **Walkthrough:** `.json().with_current_span(true)` prints one JSON object per event, including the enclosing span's fields. The writer is a closure returning a clone of a `Mutex<Vec<u8>>` wrapper (`MakeWriter` is implemented for such closures); afterwards the lines are parsed with `serde_json`. `tracing::subscriber::with_default` scopes the subscriber to a closure; `set_default` returns a guard.
- **Expected output:**
  ```
  raw output:
  {"level":"INFO","fields":{"message":"task created","task_id":42,"status":"todo"},"target":"03_json_logs"}
  {"level":"WARN","fields":{"message":"slow query","retries":2},"target":"03_json_logs","span":{"request_id":"req-0001","user_id":7,"name":"request"}}

  level="INFO" message="task created" task_id=42 span=null
  level="WARN" message="slow query" task_id=null span="req-0001"
  ```

### 3.4 Request logging middleware with correlation ids
- **Goal:** request id in, span per request, one access-log line out.
- **Run:** `ADDR=127.0.0.1:8080 cargo run -p m10-logging-and-diagnostics --example 04_request_logging`
- **Code:** [examples/04_request_logging.rs](examples/04_request_logging.rs)
- **Walkthrough:** `middleware::from_fn_with_state` runs `request_logging`: reuse a valid incoming `X-Request-ID` (so one id follows a request across services) or generate `req-0001`, open `info_span!("request", %request_id, method, path)`, run the rest with `.instrument(span.clone())`, then log status and latency at ERROR (5xx), WARN (4xx) or INFO, and echo the id in the response. The handler's own `info!("saying hello")` inherits the span fields without knowing about them. (The counter id keeps the lesson deterministic; real services use a UUID, module 13.)
- **Try it:**
  ```
  $ curl -i http://127.0.0.1:8080/hello
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  x-request-id: req-0001

  Hello, world!
  $ curl -i -H 'X-Request-ID: demo-42' http://127.0.0.1:8080/hello      # -> x-request-id: demo-42
  $ curl -i http://127.0.0.1:8080/fail                                  # -> 500, x-request-id: req-0002
  $ curl -i http://127.0.0.1:8080/nothing                               # -> 404, x-request-id: req-0003
  ```
- **Expected server log** (`latency_ms` is 0 for these loopback calls; it varies on your machine):
  ```
  INFO listening addr=127.0.0.1:8080
   INFO request{request_id=req-0001 method=GET path=/hello}: saying hello
   INFO request{request_id=req-0001 method=GET path=/hello}: request completed status=200 latency_ms=0
   INFO request{request_id=demo-42 method=GET path=/hello}: saying hello
   INFO request{request_id=demo-42 method=GET path=/hello}: request completed status=200 latency_ms=0
  ERROR request{request_id=req-0002 method=GET path=/fail}: request failed status=500 latency_ms=0
   WARN request{request_id=req-0003 method=GET path=/nothing}: request rejected status=404 latency_ms=0
   INFO shut down cleanly
  ```

### 3.5 Task Tracker diagnostics: TraceLayer, panics, health, metrics
- **Goal:** the standard tower-http stack plus panic recovery and counters.
- **Run:** `ADDR=127.0.0.1:8080 cargo run -p m10-logging-and-diagnostics --example 05_task_diagnostics`
- **Code:** [examples/05_task_diagnostics.rs](examples/05_task_diagnostics.rs)
- **Walkthrough:** `SetRequestIdLayer` (custom `MakeRequestId`; `MakeRequestUuid` is the stock alternative) assigns ids, `TraceLayer` opens the span (reading `x-request-id`) and logs each response, `PropagateRequestIdLayer` copies the id to the response, `count_requests` updates atomics, and `CatchPanicLayer::custom` converts a panic into the 500 error envelope. **Layer order matters:** `.layer()` wraps everything added before it, so the list reads innermost first. `CatchPanicLayer` is innermost, so the 500 it produces still passes through the metrics counter and `TraceLayer`. A panic hook turns the panic into a structured ERROR event with its location; the client never sees the message.
- **Try it:**
  ```
  $ curl -si http://127.0.0.1:8080/healthz                             # 200 {"status":"ok"}   x-request-id: req-0001
  $ curl -si -H 'X-Request-ID: demo-1' http://127.0.0.1:8080/tasks/1   # 200 {"id":1,"status":"todo","title":"Write docs"}
  $ curl -si http://127.0.0.1:8080/tasks/9
  HTTP/1.1 404 Not Found
  content-type: application/json
  x-request-id: req-0002

  {"error":{"code":"not_found","message":"task not found"}}
  $ curl -si http://127.0.0.1:8080/panic
  HTTP/1.1 500 Internal Server Error
  content-type: application/json
  x-request-id: req-0003

  {"error":{"code":"internal","message":"internal server error"}}
  $ curl -s http://127.0.0.1:8080/healthz                              # still serving: {"status":"ok"}
  $ curl -s http://127.0.0.1:8080/metrics
  http_requests_total 5
  http_server_errors_total 1
  ```
- **Expected server log** (`latency=0 ms` varies):
  ```
  INFO listening addr=127.0.0.1:8080
   INFO request{request_id="req-0001" method=GET path=/healthz}: finished processing request latency=0 ms status=200
   INFO request{request_id="demo-1" method=GET path=/tasks/1}: task found task_id=1
   INFO request{request_id="demo-1" method=GET path=/tasks/1}: finished processing request latency=0 ms status=200
   INFO request{request_id="req-0002" method=GET path=/tasks/9}: finished processing request latency=0 ms status=404
  ERROR request{request_id="req-0003" method=GET path=/panic}: handler panicked panic="something impossible happened" location=modules/10-logging-and-diagnostics/examples/05_task_diagnostics.rs:73
   INFO request{request_id="req-0003" method=GET path=/panic}: finished processing request latency=0 ms status=500
  ERROR request{request_id="req-0003" method=GET path=/panic}: response failed classification=Status code: 500 Internal Server Error latency=0 ms
   INFO request{request_id="req-0004" method=GET path=/healthz}: finished processing request latency=0 ms status=200
   INFO request{request_id="req-0005" method=GET path=/metrics}: finished processing request latency=0 ms status=200
   INFO shut down cleanly
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `init()` panics because a global subscriber is already installed | A subscriber can be installed once per process; tests or libraries call it twice | Install only in `main`; use `try_init()` in helpers; use `with_default`/`set_default` in tests |
| Logs vanish because the guard was dropped | `let _ = tracing::subscriber::set_default(s)` drops the guard immediately (same for non-blocking writer guards) | Bind it: `let _guard = ...;` and keep it alive for the scope |
| Wrong or missing context in async code | `let _e = span.enter();` across `.await` attaches the span to the thread, not the task; `tokio::spawn` starts a task with no span | Use `.instrument(span)` / `#[instrument]`, and instrument spawned futures |
| Secrets in logs | `#[instrument]` records every argument with `Debug`; `info!(?config)` prints whole structs | `skip`/`skip_all`, log selected fields, wrap secrets in `SecretString` (module 09) |
| `info!("user {id} created")` | Interpolation bakes values into the message; nothing to filter on | `info!(user_id = id, "user created")` |
| `RUST_LOG=debug` drowns you in hyper/h2/rustls lines | The directive applies to every crate | `info,my_app=debug`; per-target directives (3.1) |
| Panic escapes the middleware you expected | Layer order: a counter placed *outside* `CatchPanicLayer` never sees the 500; panics inside `tokio::spawn`ed tasks are never caught by the layer | Put `CatchPanicLayer` innermost; handle `JoinError` for spawned tasks |
| Flaky log-capturing tests | tracing caches, per call site, whether any subscriber is interested; parallel tests that hit the same `info!` without a subscriber can leave a capture empty | Give such tests their own test binary (`tests/logging.rs`, as the exercises do) or install a subscriber in every test |

## 5. Coming From Other Languages
- **Go:** `log/slog` is the closest relative (structured attributes, handlers), but Rust adds spans and compile-time-cheap disabled events. `context.Context` carrying a logger becomes the *current span*.
- **Python:** `logging` + `structlog`: levels and `extra=` fields map to events and fields; `contextvars` map to spans.
- **Java/C#:** SLF4J/MDC and `ILogger` scopes correspond to spans and fields; `Subscriber`s are the appenders/providers; `RUST_LOG` is the `logging.level` configuration.
- **TypeScript:** `pino` child loggers (`logger.child({ requestId })`) are spans.
- **C++:** spdlog sinks resemble `MakeWriter`; a panic is not an exception: `CatchPanicLayer` uses `catch_unwind` at the request boundary only.

## 6. Exercises
Tests are offline and fast; the logging assertions parse JSON captured in memory.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Structured events | Easy | typed fields, levels, safe `EnvFilter` construction | `cargo test -p m10-ex1-structured-events` |
| 2 | Request-id middleware | Medium | `from_fn_with_state`, extensions, spans on async code | `cargo test -p m10-ex2-request-id` |
| 3 | Diagnostics router | Stretch | `CatchPanicLayer`, layer order, metrics, no leaked panic text | `cargo test -p m10-ex3-diagnostics-router` |

**Exercise 1: log helpers** ([starter](exercises/ex1-structured-events/src/lib.rs), [tests](exercises/ex1-structured-events/tests/events.rs))
- **Task:** `log_task_created` (INFO with numeric `task_id`; title only in a DEBUG event), `log_store_error`, `log_operation` (WARN above a threshold, DEBUG otherwise) and `build_filter` (valid `RUST_LOG` or a default, never a panic).
- **Acceptance:** the title never appears at INFO; `task_id` is a JSON number; `error` is a string.
<details><summary>Hint</summary><code>task_id = task.id</code> records a number, <code>task_id = %task.id</code> a string; <code>EnvFilter::try_new</code> returns a <code>Result</code>.</details>

**Exercise 2: `request_id_middleware`** ([starter](exercises/ex2-request-id/src/lib.rs), [tests](exercises/ex2-request-id/tests/request_id.rs), [log test](exercises/ex2-request-id/tests/logging.rs))
- **Task:** `IdGenerator::next_id`, `is_valid_request_id` and the middleware: reuse a valid id, else generate; put `RequestId` in extensions; run the rest of the stack inside a span; set the response header.
- **Acceptance:** sequential ids, echo and replacement rules, and handler log events carrying `request_id` in their span.
<details><summary>Hint</summary><code>next.run(request).instrument(span).await</code>; the generator is a shared <code>AtomicU64</code>.</details>

**Exercise 3: `build_app`** ([starter](exercises/ex3-diagnostics-router/src/lib.rs), [tests](exercises/ex3-diagnostics-router/tests/diagnostics.rs), [log test](exercises/ex3-diagnostics-router/tests/logging.rs))
- **Task:** implement `Metrics::record`/`render`, `count_requests`, `panic_response` (log at ERROR, answer with the error envelope, hide the message) and add the two layers in the right order.
- **Acceptance:** a panic gives the 500 envelope, the app keeps serving, panics and 404s are counted, the panic text is only in the log.
<details><summary>Hint</summary>The counting middleware must be the OUTER layer, so add it after <code>CatchPanicLayer</code>.</details>

Reference solutions: [solutions/](solutions/) (overlay onto `exercises/`).

## 7. Recap
- Log events with typed fields; put context in spans; filter with directives, not code changes.
- Only `main` installs a subscriber; libraries just emit.
- One request id per request, accepted if sane, echoed back, present on every line.
- Convert panics into a logged 500 at the edge; mind layer order.
- Health and a handful of atomic counters go a long way before you need a metrics stack.

## 8. Self-check Questions
1. What is the difference between an event and a span, and why does a handler's `info!` show the request id without mentioning it?
2. Why is `let _guard = span.enter();` inside an `async fn` a bug, and what do you write instead?
3. In `.layer(A).layer(B)`, which one sees the request first? What breaks if the metrics layer is inner and `CatchPanicLayer` outer?
4. Why must the incoming `X-Request-ID` be validated before it is logged and echoed?
5. Which directive shows `debug` logs for your crate only, and why is plain `RUST_LOG=debug` a poor default?

## 9. Further Reading
- [`tracing` documentation](https://docs.rs/tracing) and the [tokio tracing guide](https://tokio.rs/tokio/topics/tracing)
- [`tracing-subscriber` EnvFilter directives](https://docs.rs/tracing-subscriber/latest/tracing_subscriber/filter/struct.EnvFilter.html)
- [`tower-http`](https://docs.rs/tower-http): `trace`, `request_id`, `catch_panic`

---
**Next:** [11 Authentication and Security Basics](../11-auth-and-security-basics/README.md)
