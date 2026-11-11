# P1: Hello HTTP

A tiny HTTP server with routing, middleware (request ID, access log, panic recovery), timeouts and **graceful shutdown**, built with `axum`.

**Modules exercised:** [01 Language fundamentals](../../modules/01-language-fundamentals/README.md), [02 HTTP server](../../modules/02-http-server/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md).

## Requirements

| Route | Behaviour |
|-------|-----------|
| `GET /healthz` | `200` `{"status":"ok"}` (`application/json`) |
| `GET /hello` | `200` `text/plain` `Hello, world!` |
| `GET /hello/{name}` | `200` `text/plain` `Hello, {name}!` (percent-decoded) |
| `GET /slow?ms=N` | sleeps `min(N, 2000)` ms, then `200` `done`; `ms` defaults to `0`; non-numeric `ms` -> `400` `bad_request` envelope |
| `GET /panic` | the handler panics (already written); the client gets `500` with the error envelope, the server keeps serving |

Error envelope (JSON, `error::error_response` already builds it):
`{"error":{"code":"internal","message":"internal server error"}}`. The panic message must never reach the client.

Middleware (outermost first):

1. **Request ID**: echo a client `X-Request-ID` (non-empty, at most 128 bytes), otherwise generate a UUID v4. Present on every response, including panics and timeouts.
2. **Access log**: exactly one line per request, sent to the injectable `Config::log_sink`:
   `method=GET path=/hello/Ferris status=200 duration_ms=3 request_id=<id>`
3. **Panic recovery**: `CatchPanicLayer` with a handler that produces the `500` envelope.
4. **Timeout**: requests slower than `Config::request_timeout` (default 10 s) get `408`.

Lifecycle: `run(listener, app, shutdown)` serves until `shutdown` resolves, then stops accepting connections, lets in-flight requests finish and returns. `main` reads `ADDR` (default `127.0.0.1:8080`) and wires Ctrl-C / SIGTERM to `shutdown`.

## Acceptance criteria (test names in `starter/tests/hello_http.rs`)

| Area | Tests |
|------|-------|
| Health | `healthz_returns_status_ok_json` |
| Hello | `hello_without_name_greets_world`, `hello_with_name_greets_that_name`, `hello_is_plain_text`, `hello_decodes_percent_encoded_names` |
| Slow | `slow_waits_then_answers_done`, `slow_without_ms_answers_immediately`, `slow_caps_the_delay_at_two_seconds`, `slow_with_non_numeric_ms_is_a_bad_request_envelope` |
| Panic | `panic_route_answers_500_with_error_envelope`, `panic_message_is_not_leaked_to_the_client`, `server_keeps_serving_after_a_panic` |
| Request ID | `request_id_is_generated_when_missing`, `request_id_from_client_is_echoed`, `oversized_client_request_id_is_replaced`, `request_id_is_present_on_panic_responses` |
| Access log | `every_request_produces_exactly_one_log_line`, `log_line_has_method_path_status_and_duration`, `log_line_carries_the_request_id`, `panicking_request_is_logged_with_status_500` |
| Timeout | `requests_exceeding_the_timeout_get_408` |
| Shutdown | `shutdown_lets_in_flight_request_finish`, `run_returns_after_shutdown_once_requests_are_done`, `new_connections_are_refused_after_shutdown` |

## How to run

Work in `projects/P1-hello-http/starter/` (all commands from `languages/rust/`):

```bash
cargo test -p p1-hello-http                  # starter: all tests fail until you implement the TODOs
ADDR=127.0.0.1:8080 cargo run -p p1-hello-http
curl -i localhost:8080/hello/Ferris
curl -i localhost:8080/panic
```

Stop the server with Ctrl-C; try `curl 'localhost:8080/slow?ms=1500'` in another terminal and press Ctrl-C while it is waiting: the response still arrives.

## Where the TODOs are

| File | Work |
|------|------|
| `src/handlers.rs` | `healthz`, `hello_world`, `hello`, `slow` |
| `src/middleware.rs` | `request_id`, `access_log`, `panic_response` |
| `src/lib.rs` | add the layers in `build_app`; graceful shutdown in `run` |

<details>
<summary>Hints</summary>

- `Json(serde_json::json!({...}))`, `String` and `&'static str` all implement `IntoResponse`.
- `Query<T>` rejections are plain text; take `Result<Query<T>, QueryRejection>` and map the `Err` yourself.
- `tokio::time::sleep` (never `std::thread::sleep`, it blocks a runtime thread).
- Middleware with `axum::middleware::from_fn` / `from_fn_with_state`: `async fn(State<_>, Request, Next) -> Response`; read what you need from the request *before* `next.run(req)`.
- Layers: `Router::layer` wraps everything added so far; the **last** `.layer(..)` call is the **outermost**. Put the request ID outside the access log, the access log outside the panic catcher.
- `tower_http::catch_panic::CatchPanicLayer::custom(fn)` and `tower_http::timeout::TimeoutLayer::with_status_code(408, dur)`.
- `axum::serve(listener, app).with_graceful_shutdown(signal)`.
- `HeaderValue::len()`/`is_empty()` are available; `uuid::Uuid::new_v4()` makes the id.

</details>

## Stretch goals (not tested)

- Replace the hand-written log line with `tracing` events and a `TraceLayer` (module 10).
- Add a header-read timeout at the connection level (needs `hyper-util` directly; `axum::serve` does not expose it).
- Add a drain timeout: abort `run` if in-flight requests do not finish within N seconds.
- Count requests with an `AtomicU64` and expose `GET /metrics`.

## Reference solution

`solution/` contains only the files that differ from `starter/`, at the same relative paths (`src/lib.rs`, `src/handlers.rs`, `src/middleware.rs`). Try on your own first; to check your work, compare against it or overlay it on a scratch copy of `starter/`.
