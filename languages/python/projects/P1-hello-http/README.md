# P1: Hello HTTP

**Modules exercised:** [01 Language fundamentals](../../modules/01-language-fundamentals/README.md), [02 HTTP server](../../modules/02-http-server/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md).

## Goal

Build a tiny FastAPI service that has the pieces every production service needs: routing, three middlewares (request id, access log, panic recovery), the shared error envelope, server timeouts and a **graceful shutdown** that lets in-flight requests finish.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | `200` `{"status":"ok"}` (already done in the starter) |
| `GET /hello` | `200` `text/plain` `Hello, world!` |
| `GET /hello/{name}` | `200` `text/plain` `Hello, {name}!` (the name is percent-decoded) |
| `GET /slow?ms=N` | waits `N` ms (`0..2000`, default `100`), then `200` `done`. Any other value is a `400` `bad_request` envelope |
| `GET /panic` | the handler raises; the client gets `500` with the `internal` envelope (no details) and the server keeps serving |
| anything else | `404` with the `not_found` envelope (already done) |

Error envelope: `{"error":{"code":"internal","message":"internal server error"}}`, `Content-Type: application/json`.

Middleware (outermost first):

1. **Request id**: echo an incoming `X-Request-ID` if it is 1-64 characters of `[A-Za-z0-9._-]`, otherwise generate one (`uuid4().hex`). The id is on every response, errors included.
2. **Access log**: exactly one line per request to the injected sink (`Config.log_sink`), format `method=GET path=/healthz status=200 duration_ms=0.42 request_id=<id>`. The path is percent-encoded so a client cannot forge extra log lines.
3. **Recover**: an unhandled exception is logged with its traceback (`logger.exception`) and becomes the `500` envelope.

Server: `make_server(app)` sets a keep-alive timeout (at most 30 s) and a bounded `timeout_graceful_shutdown`. `run(app, sock, shutdown)` serves on an already bound socket until the `asyncio.Event` is set; requests in flight finish, the listening socket closes at once (new connections are refused) and `run` returns after the last response.

`Config` makes the service testable: `log_sink` receives the log lines and `sleep` is what `/slow` awaits, so tests never really sleep.

## Run

```bash
python -m pytest projects/P1-hello-http/tests      # tests (from languages/python)
cd projects/P1-hello-http/starter
ADDR=127.0.0.1:8080 python -m p1_hello_http         # default ADDR is 127.0.0.1:8080
```

```bash
curl -i localhost:8080/hello/Ada
curl -i -H 'X-Request-ID: demo-1' localhost:8080/healthz
curl -i localhost:8080/panic
curl "localhost:8080/slow?ms=1500"   # press Ctrl-C in the server terminal meanwhile: this still answers "done"
```

Ctrl-C and SIGTERM both start the graceful shutdown.

## Acceptance criteria (tests)

| Criterion | Tests (`tests/test_p1_http.py`, `tests/test_p1_shutdown.py`) |
|-----------|--------------------------------------------------------------|
| health | `test_healthz_returns_ok_json` |
| hello | `test_hello_greets_the_name_as_plain_text`, `test_hello_without_a_name_greets_the_world`, `test_hello_decodes_percent_encoded_names` |
| 404 envelope | `test_unknown_route_returns_not_found_envelope` |
| slow | `test_slow_waits_the_requested_milliseconds_then_says_done`, `test_slow_accepts_the_maximum_of_2000_ms`, `test_slow_rejects_bad_ms_with_a_400_envelope` |
| panic | `test_panic_returns_a_500_envelope_without_details`, `test_server_keeps_serving_after_a_panic` |
| request id | `test_request_id_is_generated_when_missing`, `test_request_id_from_the_client_is_echoed`, `test_unsafe_request_id_is_replaced`, `test_request_id_is_present_on_error_responses` |
| access log | `test_access_log_writes_one_line_per_request`, `test_access_log_line_has_method_path_status_and_duration`, `test_access_log_includes_the_request_id`, `test_access_log_records_status_500_for_a_panic`, `test_access_log_cannot_be_split_by_a_newline_in_the_path` |
| timeouts | `test_server_timeouts_are_configured` |
| graceful shutdown | `test_in_flight_request_finishes_and_new_connections_are_refused_after_shutdown`, `test_run_serves_requests_until_shutdown_is_requested` |

## What to implement

The starter runs and answers `/healthz`; every `# TODO` is yours:

| File | Work |
|------|------|
| `starter/p1_hello_http/app.py` | `hello_world`, `hello`, `slow` (validation with `Query(ge=, le=)`), `panic` |
| `starter/p1_hello_http/middleware.py` | `request_id`, `access_log`, `recover` |
| `starter/p1_hello_http/server.py` | timeouts in `make_server`, `run` |

Tests import `p1_hello_http` from `starter/`; the solution is overlaid on the same files.

<details>
<summary>Hints</summary>

- `app.middleware("http")(fn)` registers a function; the **last** registered middleware is the **outermost**. `create_app` already registers them in the right order.
- `request.state` is shared by all middlewares of one request: set `request.state.request_id` in `request_id` and read it in `access_log`.
- `time.perf_counter()` measures durations; `urllib.parse.quote(path)` escapes control characters.
- Exceptions raised by a route reach `recover` through `await call_next(request)`. FastAPI's `ServerErrorMiddleware` sits *outside* your middlewares, so an exception handler for `Exception` would skip the request id and the log line.
- `run`: `asyncio.create_task(server.serve([sock]))`, wait for `shutdown.wait()` (or the server task) with `asyncio.wait(..., return_when=FIRST_COMPLETED)`, then set `server.should_exit = True` and `await` the server task. `_Server` already stops uvicorn from installing its own signal handlers.
- `uvicorn.Config(timeout_keep_alive=..., timeout_graceful_shutdown=...)`.

</details>

## Stretch goals (not tested)

- Reject requests whose body or headers are too large (`h11_max_incomplete_event_size`) and explain why uvicorn has no read-header timeout (put a reverse proxy in front).
- Add a `/metrics` counter of requests per status code, protected against races.
- Emit the access log as JSON through the `logging` module with a `contextvars` request id (module 10).

## Reference solution

[`solution/`](./solution/) holds only the three files that differ from the starter (`app.py`, `middleware.py`, `server.py`). Try the project first, then compare.
