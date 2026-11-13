# 10. Logging and Diagnostics in Python

> Estimated time: 1.5-2 h | Difficulty: Intermediate | Prerequisites: [02 HTTP server](../02-http-server/README.md), [06 Concurrency](../06-concurrency/README.md) (`contextvars`), [09 Configuration](../09-configuration/README.md)

## Learning Outcomes

After this module you can:

- [ ] Configure the stdlib `logging` tree (loggers, levels, handlers, formatters) once, at application startup
- [ ] Emit structured (JSON) log lines with contextual fields via `extra=`
- [ ] Attach a correlation id to every log line of a request with `contextvars`
- [ ] Write ASGI middleware that logs one line per request and turns crashes into a safe `500`
- [ ] Expose health and basic metrics endpoints
- [ ] Use `tracemalloc`, `cProfile` and `faulthandler` to investigate memory, CPU and hangs

## 1. Concept Overview

Logs are the black box recorder of a service: when something breaks at 3 a.m. they are usually all you have. Good logs are **structured** (machine-parseable, one event per line), **contextual** (every line carries the request id, user id or job name that caused it) and **safe** (no passwords, tokens or stack traces leaked to clients).

A request travels like this, and each layer adds something:

```text
client -> [request-id + timing + recovery middleware] -> handler -> log calls
              |  sets request_id (contextvar)             |  every record is stamped with it
              +-> one access line: method, path, status, duration_ms
```

Alongside logs, a service exposes **health** (`/healthz`: "the process is alive") and **metrics** (`/metrics`: counters such as `requests_total`), and keeps profilers at hand for the day it is slow or leaking memory.

## 2. How Python Approaches It

| Tool | Use |
|------|-----|
| `logging.getLogger(__name__)` | One logger per module; names form a dotted tree (`tracker.api` is a child of `tracker`) |
| `Handler` + `Formatter` | Where lines go (stream, file, rotating file) and how they look |
| `Logger.setLevel` / handler level | Two gates: the logger decides if a record is created, each handler decides if it prints it |
| `extra={...}` | Adds attributes to the `LogRecord`; a JSON formatter turns them into fields |
| `logging.Filter` | Mutates or drops records, ideal for stamping the request id |
| `contextvars.ContextVar` | Per-request state that follows `await` and `asyncio` tasks (a global would mix requests) |
| `tracemalloc`, `cProfile`, `faulthandler`, `python -X dev` | Memory, CPU, crash/hang diagnostics from the stdlib |

Idioms to follow:

- **Libraries create loggers, applications configure them.** A library adds only a `NullHandler`; the app calls `basicConfig` (or builds handlers) once in `main()`.
- **Lazy formatting:** `log.info("user %s", user_id)`, not an f-string. The message is built only if the record is emitted, and tools can group identical templates.
- **`log.exception(...)`** inside `except` logs at `ERROR` with the traceback; `log.error("...", exc_info=True)` does the same elsewhere.
- The stdlib has no JSON output built in, so we write a 20-line `JsonFormatter` ([`m10_helpers.py`](./examples/m10_helpers.py)). [`structlog`](https://www.structlog.org/) is the popular third-party alternative; it is not used here.

## 3. Guided Examples

### 3.1 Loggers, levels, handlers

- **Goal:** understand the logger tree, level gates and where configuration belongs.
- **Run:** `python modules/10-logging-and-diagnostics/examples/01_logging_basics.py`
- **Code:** [examples/01_logging_basics.py](./examples/01_logging_basics.py)
- **Walkthrough:** `configure()` adds one stdout handler to the **root** logger and sets `INFO`. Child loggers such as `tracker.api` have no handler of their own: records *propagate* up to the root handler. `tracker.store` gets its own `DEBUG` level, so its debug line passes while `tracker.api`'s is dropped. `log.exception` appends the exception (shortened here by a custom `formatException` so the output is stable; real services keep the full traceback).

```python
log.info("server starting on %s:%d", "127.0.0.1", 8080)  # lazy %-formatting
log.exception("lookup failed")  # ERROR + traceback, only inside `except`
```

- **Expected output:**

  ```text
  INFO    tracker.api: server starting on 127.0.0.1:8080
  WARNING tracker.api: slow query took 412.3 ms
  DEBUG   tracker.store: kept: this logger has its own DEBUG level
  ERROR   tracker.api: lookup failed
  KeyError: 'port'
  INFO    tracker.api: DEBUG enabled for tracker.api? False
  ```

### 3.2 Structured JSON logs

- **Goal:** one JSON object per line, with contextual fields.
- **Run:** `python modules/10-logging-and-diagnostics/examples/02_json_logging.py`
- **Code:** [examples/02_json_logging.py](./examples/02_json_logging.py), formatter in [examples/m10_helpers.py](./examples/m10_helpers.py)
- **Walkthrough:** `JsonFormatter.format` starts from `level`, `logger`, `message` and copies every record attribute that is *not* a standard `LogRecord` attribute: exactly the fields passed with `extra=`. A `LoggerAdapter(..., merge_extra=True)` stamps the same fields (`job`) on every call of one unit of work. Without `merge_extra=True` the per-call `extra=` would be silently dropped.
- **Expected output:**

  ```text
  {"level": "info", "logger": "tracker.tasks", "message": "task created", "task_id": 7, "status": "todo", "tags": ["docs", "p1"]}
  {"level": "warning", "logger": "tracker.tasks", "message": "rejected empty title", "field": "title"}
  {"level": "info", "logger": "tracker.tasks", "message": "started", "job": "nightly-export"}
  {"level": "info", "logger": "tracker.tasks", "message": "finished", "job": "nightly-export", "rows": 120}
  {"level": "error", "logger": "tracker.tasks", "message": "bad number", "input": "abc", "exception": "ValueError: invalid literal for int() with base 10: 'abc'"}
  ```

### 3.3 Correlation ids with `contextvars`

- **Goal:** give every concurrent request its own id without passing it around.
- **Run:** `python modules/10-logging-and-diagnostics/examples/03_request_context.py`
- **Code:** [examples/03_request_context.py](./examples/03_request_context.py)
- **Walkthrough:** each `asyncio` task runs in a *copy* of the context, so `request_id_var.set(...)` in `req-a` is invisible to `req-b` even though they interleave at every `await`. A `RequestIdFilter` on the handler copies the variable onto each record, and the format string uses `%(request_id)s`. Always `reset(token)` in `finally`.
- **Expected output:**

  ```text
  [req-a] step 1 of 2
  [req-b] step 1 of 3
  [req-a] step 2 of 2
  [req-b] step 2 of 3
  [req-b] step 3 of 3
  [None] outside any request
  ```

### 3.4 Diagnostics: memory, CPU, hangs

- **Goal:** know which stdlib tool answers which question.
- **Run:** `python modules/10-logging-and-diagnostics/examples/04_diagnostics.py`
- **Code:** [examples/04_diagnostics.py](./examples/04_diagnostics.py)
- **Walkthrough:** `tracemalloc.start()` then `take_snapshot().statistics("lineno")` shows *which line* allocates most; `cProfile.Profile` records time per function (`pstats` sorts and prints it); `faulthandler.enable()` prints every thread's stack when the interpreter crashes, and `dump_traceback_later(timeout)` is a watchdog for hangs. Also useful: `python -X dev` (extra runtime checks, `ResourceWarning`s), `breakpoint()` and `python -m pdb`. The example prints only stable facts, not timings.
- **Expected output:**

  ```text
  index holds 50000 entries
  tracemalloc saw allocations: True
  biggest allocation site: 04_diagnostics.py
  matches found: 1000
  profiler recorded slow_search: True
  faulthandler enabled: True
  ```

### 3.5 Task Tracker service: request logging, recovery, metrics

- **Goal:** the full stack in a FastAPI service.
- **Run:** `ADDR=127.0.0.1:8080 python modules/10-logging-and-diagnostics/examples/05_tracker_service.py` (stop with Ctrl-C or SIGTERM; `LOG_LEVEL` selects the level)
- **Code:** [examples/05_tracker_service.py](./examples/05_tracker_service.py)
- **Walkthrough:** `RequestLogMiddleware` is a **pure ASGI** middleware (not `BaseHTTPMiddleware`), so ContextVars set in it are visible to handlers and it can wrap `send` to capture the status and add `X-Request-ID`. It accepts a client id only if it matches `[A-Za-z0-9._-]{1,64}` (otherwise a client could inject newlines or megabytes into your logs). Exceptions from handlers are logged with their traceback and replaced by the shared `internal` envelope. The response never contains the exception text. Uvicorn is started with `log_config=None` and `access_log=False` so its records flow into our JSON handler and the access line is written once.

```python
try:
    await self.app(scope, receive, send_with_id)
except Exception:
    log.exception("unhandled error")  # full traceback goes to the log only
    if not response_started:
        await JSONResponse(body, status_code=500)(scope, receive, send_with_id)
```

- **Expected output:** from another terminal

  ```console
  $ curl -si -H 'X-Request-ID: demo-1' localhost:8080/healthz
  HTTP/1.1 200 OK
  server: uvicorn
  content-length: 15
  content-type: application/json
  x-request-id: demo-1

  {"status":"ok"}
  $ curl -s -X POST -H 'Content-Type: application/json' -d '{"title":"Write docs"}' localhost:8080/tasks
  {"id":1,"title":"Write docs","status":"todo"}
  $ curl -si -H 'X-Request-ID: demo-3' localhost:8080/panic
  HTTP/1.1 500 Internal Server Error
  server: uvicorn
  content-length: 63
  content-type: application/json
  x-request-id: demo-3

  {"error":{"code":"internal","message":"internal server error"}}
  $ curl -s localhost:8080/metrics
  requests_total 3
  errors_total 1
  ```

  The server's stdout (`time`, `duration_ms` and generated ids vary; the traceback of `/panic` is in the `exception` field):

  ```json
  {"time": "2026-01-01T09:00:00.125436+00:00", "level": "info", "logger": "tracker.access", "message": "request", "method": "GET", "path": "/healthz", "status": 200, "duration_ms": 17.67, "request_id": "demo-1"}
  {"time": "2026-01-01T09:00:00.132532+00:00", "level": "info", "logger": "tracker.api", "message": "task created", "task_id": 1, "request_id": "demo-2"}
  {"time": "2026-01-01T09:00:00.144761+00:00", "level": "info", "logger": "tracker.access", "message": "request", "method": "GET", "path": "/panic", "status": 500, "duration_ms": 6.82, "request_id": "demo-3"}
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Calling `logging.basicConfig()` in a library or at import time | It looks like the quick way to "turn logging on" | Libraries only `getLogger(__name__)` (+ `NullHandler`); the app configures once in `main()`. `basicConfig` does nothing if the root logger already has handlers (use `force=True` deliberately) |
| `log.info(f"user {user} paid {amount}")` | f-strings feel natural | `log.info("user %s paid %s", user, amount)`: lazy, groupable, and no work when the level is off |
| `except Exception as e: log.error(str(e))` | The traceback is lost | `log.exception("what failed")` or `exc_info=True`; re-raise or translate, never swallow |
| Logging passwords, tokens, `Authorization` headers or whole request bodies | `extra={"body": body}` is convenient | Log ids and field names, not secrets; redact in the formatter as a safety net |
| `extra={"name": ...}` or `extra={"message": ...}` raises `KeyError: "Attempt to overwrite 'name' in LogRecord"` | `extra` keys must not collide with `LogRecord` attributes (`name`, `args`, `msg`, `module`...) | Use domain-specific keys (`task_name`, `user_id`) |
| A module-level global `current_request_id` | Works in a single-user test, mixes up concurrent requests | `ContextVar` (and `reset(token)`); threads started with `threading.Thread` do **not** inherit the context, use `contextvars.copy_context().run` |
| `BaseHTTPMiddleware` for logging and ids | It is the documented, easy option | Prefer pure ASGI middleware: no extra task per request, ContextVars propagate, streaming responses are untouched |
| A blocking `FileHandler` in an `async def` hot path | Logging looks free | Handlers do synchronous I/O; log to stdout (let the platform collect it) or use `QueueHandler` + `QueueListener` |
| Trusting a client `X-Request-ID` blindly | "Echoing is the feature" | Validate length and charset, otherwise generate your own |
| Returning `str(exc)` in the 500 body | Helpful while developing | Return the fixed `internal` envelope; the detail goes to the log |

## 5. Coming From Other Languages

- **Java/C# (SLF4J/Serilog, `ILogger<T>`):** the logger tree and levels are the same idea, but configuration is global and imperative (`basicConfig`/`dictConfig`) and there is no DI: each module calls `getLogger(__name__)`. `extra=` plays the role of structured properties (MDC in Java).
- **Go (`slog`):** `slog.With(...)` returns a new logger with fields; in Python use `LoggerAdapter` or a `Filter` + `ContextVar`. Go threads a `context.Context` through calls, Python's `ContextVar` is ambient.
- **JavaScript/TypeScript (pino):** pino emits JSON by default; the Python stdlib needs a formatter. `AsyncLocalStorage` is the equivalent of `ContextVar`.
- **Panics:** an uncaught exception in a handler is Python's "panic"; the middleware above is the recovery layer (Go `recover`, Express error middleware).

## 6. Exercises

Each exercise has starter code and tests in [exercises/](./exercises/). Solutions are in [solutions/](./solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [JSON formatter](./exercises/ex1_json_formatter/m10_json_formatter.py) | Easy | `Formatter`, `extra=`, `exc_info` | `pytest modules/10-logging-and-diagnostics/exercises/ex1_json_formatter` |
| 2 | [Request context](./exercises/ex2_request_context/m10_request_context.py) | Medium | `ContextVar`, `Filter`, context managers | `pytest modules/10-logging-and-diagnostics/exercises/ex2_request_context` |
| 3 | [Access-log middleware](./exercises/ex3_access_log/m10_access_log.py) | Stretch | ASGI, request ids, timing, recovery | `pytest modules/10-logging-and-diagnostics/exercises/ex3_access_log` |

**Exercise 1: JSON formatter**

- **Task:** implement `JsonFormatter.format`: `level`, `logger`, `message`, the `extra=` fields and, for exceptions, an `exception` field, all on one line; unserialisable values fall back to `str()`.
- **Acceptance:** all tests in `test_m10_json_formatter.py`, including "no standard attributes leak" and "date and set do not crash the logger".
- <details><summary>Hint</summary>Compare `vars(record)` with the attributes of an empty record made by `logging.makeLogRecord({})`; `json.dumps(..., default=str)`.</details>

**Exercise 2: Request context**

- **Task:** implement `current_request_id`, the `request_context` context manager (generate an id when none is given, restore the previous value on exit, also on errors) and `RequestIdFilter`.
- **Acceptance:** nesting, error-safety, injected id factory and isolation between `asyncio` tasks.
- <details><summary>Hint</summary>`ContextVar.set` returns a token; hand it to `ContextVar.reset` in a `finally`.</details>

**Exercise 3: Access-log middleware**

- **Task:** implement the pure ASGI `AccessLogMiddleware` as described in the module docstring: request id in and out, one `request` record with `duration_ms` from an injected clock, and 500-envelope recovery that logs the traceback.
- **Acceptance:** all tests in `test_m10_access_log.py`. They use FastAPI's `TestClient` and pytest's `caplog`.
- <details><summary>Hint</summary>Wrap `send` to read the status from the `http.response.start` message and to add the header; log in a `finally` so failed requests are logged too.</details>

## 7. Recap

- Libraries create loggers; the application configures handlers, formatters and levels exactly once.
- Log events, not sentences: lazy `%s` arguments plus `extra=` fields rendered as JSON.
- A `ContextVar` + `Filter` stamps every line with the request id; pure ASGI middleware sets it and writes the one access line.
- Recovery middleware logs the traceback and answers with a fixed `internal` envelope.
- Reach for `logging` first, `tracemalloc`/`cProfile`/`faulthandler` when logs are not enough, and `structlog` only when the team wants more than a 20-line formatter.

## 8. Self-check Questions

1. A child logger has level `DEBUG` but the root handler is set to `INFO`. Is its debug record printed? Which two gates does a record pass?
2. Why does `log.info("n=%d", n)` beat `log.info(f"n={n}")` even when the level is enabled?
3. Two requests are served concurrently by one event loop. Why does a module-level variable fail to identify the request while a `ContextVar` works?
4. Why does the 500 response body say only `internal server error`, and where does the traceback go?
5. When is `faulthandler.dump_traceback_later` more useful than a log line?

## 9. Further Reading

- [logging: Logging facility for Python](https://docs.python.org/3/library/logging.html) and the [Logging Cookbook](https://docs.python.org/3/howto/logging-cookbook.html) (structured logging, `QueueHandler`, contextual information)
- [contextvars](https://docs.python.org/3/library/contextvars.html), [tracemalloc](https://docs.python.org/3/library/tracemalloc.html), [The Python Profilers](https://docs.python.org/3/library/profile.html), [faulthandler](https://docs.python.org/3/library/faulthandler.html)
- [Starlette: pure ASGI middleware](https://www.starlette.io/middleware/#pure-asgi-middleware)

---
**Next:** [11 Authentication and Security Basics](../11-auth-and-security-basics/README.md)
