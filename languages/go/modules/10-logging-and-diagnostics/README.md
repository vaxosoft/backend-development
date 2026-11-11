# 10. Logging and Diagnostics in Go

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 09

## Learning Outcomes
After this module you can:
- [ ] Emit structured logs with `log/slog`: levels, key-value attributes, `With`, groups, JSON or text.
- [ ] Add a correlation (request) ID to every log line of a request through the `context`.
- [ ] Write request-logging and panic-recovery middleware that keeps the server alive and never leaks internals.
- [ ] Expose liveness, readiness and simple metrics endpoints.
- [ ] Expose `pprof` and `expvar` safely and explain why they must not sit on the public port.
- [ ] Keep secrets and personal data out of logs.

## 1. Concept Overview
Logs, metrics and profiles answer *"what happened?"*, *"how much / how often?"* and *"why is it slow?"*. **Structured** logs (key=value or JSON, one event per line) can be filtered and aggregated by machines; a **correlation ID** attached to every line of one request lets you reassemble that request's story across handlers and services. **Health endpoints** let an orchestrator decide when to restart (liveness) or route traffic (readiness). **Recovery middleware** makes sure one bad request cannot crash the process.

```
request -> [request ID] -> [access log] -> [recover] -> handler -> response
              |                |               |
              +---- ctx -------+---- logs -----+--> stdout (JSON) -> collector
                                   /metrics /healthz /readyz      (public port)
                                   /debug/pprof /debug/vars       (admin port, loopback)
```

## 2. How Go Approaches It

| Piece | Role |
|-------|------|
| `log/slog` | standard structured logger: `Logger`, `Handler` (`TextHandler`, `JSONHandler`), `Level`/`LevelVar`, `Attr`, `LogValuer` |
| `context.Context` | carries the request ID; log with `InfoContext(ctx, ...)` so handlers can read it |
| middleware `func(http.Handler) http.Handler` | request ID, access log, recover |
| `http.ResponseWriter` wrapper + `Unwrap()` | capture status/size without breaking `Flush` (`http.ResponseController`) |
| `recover` + `runtime/debug.Stack` | turn panics into a 500 and a log record with the stack |
| `expvar`, plain-text counters | basic metrics without dependencies (Prometheus clients are out of scope) |
| `net/http/pprof` | CPU/heap/goroutine profiles, `go tool pprof` |

Idiomatic: build one logger in `main`, pass it (or the pieces it needs) explicitly; attributes as typed key-values; `slog.LogValuer` types to control what is printed; decorate a `slog.Handler` to add context values. Non-idiomatic: `log.Printf` with formatted strings, the global default logger everywhere, `fmt.Println` debugging, concatenating IDs into the message text. Standard library only.

## 3. Guided Examples
Run from `languages/go/`. Examples 1-4 are deterministic programs (timestamps/durations are removed with `ReplaceAttr` for the docs, which you would not do in production).

### 3.1 slog basics
- **Goal:** handlers, levels, attributes, `With`/`WithGroup`, `LogValuer`, dynamic level.
- **Run:** `go run ./modules/10-logging-and-diagnostics/examples/01_slog_basics`
- **Code:** [examples/01_slog_basics/main.go](examples/01_slog_basics/main.go)
```go
js := slog.New(slog.NewJSONHandler(os.Stdout, opts))
reqLog := js.With("request_id", "r-42").WithGroup("http")
reqLog.Info("handled", "method", "GET", "status", 200)

func (u user) LogValue() slog.Value { // the password can never be printed
	return slog.GroupValue(slog.Int("id", u.ID), slog.String("email", u.Email))
}
```
- **Walkthrough:** the default level is `INFO`, so `Debug` is dropped. `With` pre-attaches attributes; `WithGroup` nests later ones. A `LogValuer` decides how a type appears. A `slog.LevelVar` lets you change the level while running.
- **Expected output:**
  ```
  level=INFO msg="task created" id=7 title="Write docs"
  level=WARN msg="slow query" duration=1.5s
  level=ERROR msg="save failed" err="disk full"
  {"level":"INFO","msg":"task created","id":7,"tags":["docs","p1"]}
  {"level":"INFO","msg":"handled","request_id":"r-42","http":{"method":"GET","status":200}}
  {"level":"INFO","msg":"typed","n":3,"db":{"name":"tasks","rows":2}}
  {"level":"INFO","msg":"login","user":{"id":1,"email":"a@example.com"}}
  level=DEBUG msg="visible now"
  ```

### 3.2 Request ID through the context
- **Goal:** every log line of a request carries `request_id` without passing it around.
- **Run:** `go run ./modules/10-logging-and-diagnostics/examples/02_context_request_id`
- **Code:** [examples/02_context_request_id/main.go](examples/02_context_request_id/main.go)
```go
type contextHandler struct{ slog.Handler }
func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" { r.AddAttrs(slog.String("request_id", id)) }
	return h.Handler.Handle(ctx, r)
}
```
- **Walkthrough:** the middleware reuses the caller's `X-Request-ID` or generates one, stores it in the context and echoes it in the response. The wrapper handler reads it on every `*Context` call, also in helpers deep in the call stack. `WithAttrs`/`WithGroup` must re-wrap, otherwise `logger.With(...)` silently drops the decoration. The last line shows the trap: `logger.Info` (no ctx) has no `request_id`.
- **Expected output:**
  ```
  {"level":"INFO","msg":"loading task","path":"/tasks/1","request_id":"client-abc"}
  {"level":"INFO","msg":"query executed","component":"store","rows":1,"request_id":"client-abc"}
  response header X-Request-ID: client-abc
  {"level":"INFO","msg":"loading task","path":"/tasks/1","request_id":"gen-1"}
  {"level":"INFO","msg":"query executed","component":"store","rows":1,"request_id":"gen-1"}
  response header X-Request-ID: gen-1
  {"level":"INFO","msg":"no context, no request_id"}
  ```

### 3.3 Access log and panic recovery
- **Goal:** middleware that records status/size and survives panics.
- **Run:** `go run ./modules/10-logging-and-diagnostics/examples/03_logging_recover_middleware`
- **Code:** [examples/03_logging_recover_middleware/main.go](examples/03_logging_recover_middleware/main.go)
```go
defer func() {
	v := recover()
	if v == nil { return }
	if v == http.ErrAbortHandler { panic(v) }
	logger.ErrorContext(r.Context(), "panic recovered", "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
	w.WriteHeader(http.StatusInternalServerError) // + JSON error envelope
}()
```
- **Walkthrough:** `statusRecorder` wraps the writer to learn the status (implicit `200` on first `Write`) and bytes, and exposes `Unwrap()` so `http.ResponseController` still reaches `Flush`. Logging is the *outer* middleware so it sees the `500` written by `Recover`. Log level follows status class: 5xx error, 4xx warn. The client gets the standard error envelope, never the panic text; the server keeps serving afterwards (last request).
- **Expected output** (the `stack` attribute is hidden here):
  ```
  level=INFO msg=request method=GET path=/ok status=200 bytes=4
  -> /ok 200 fine
  level=WARN msg=request method=GET path=/missing status=404 bytes=19
  -> /missing 404 404 page not found
  level=ERROR msg="panic recovered" panic=kaboom
  level=ERROR msg=request method=GET path=/boom status=500 bytes=64
  -> /boom 500 {"error":{"code":"internal","message":"internal server error"}}
  level=ERROR msg="panic recovered" panic="assignment to entry in nil map"
  level=ERROR msg=request method=GET path=/nilmap status=500 bytes=64
  -> /nilmap 500 {"error":{"code":"internal","message":"internal server error"}}
  level=INFO msg=request method=GET path=/ok status=200 bytes=4
  -> /ok 200 fine
  ```

### 3.4 Health, readiness and metrics
- **Goal:** the three operational endpoints and a drain-friendly readiness flag.
- **Run:** `go run ./modules/10-logging-and-diagnostics/examples/04_health_and_metrics`
- **Code:** [examples/04_health_and_metrics/main.go](examples/04_health_and_metrics/main.go)
- **Walkthrough:** `/healthz` (liveness) only says "the process answers"; do not check the database there or an outage restarts every instance. `/readyz` flips with an `atomic.Bool`: `503` while starting and again during shutdown so the load balancer stops sending traffic *before* `Shutdown`. Counters use `expvar.Map` (private instance) and `atomic` values; the `in_flight 1` line is the `/metrics` request itself.
- **Expected output:**
  ```
  GET /healthz  -> 200 {"status":"ok"}
  GET /readyz   -> 503 not ready
  GET /readyz   -> 200 ready
  GET /work     -> 200 done
  GET /work     -> 200 done
  GET /metrics  -> 200 requests_total 6
  in_flight 1
  GET /readyz   -> 503 not ready
  expvar JSON: {"requests_total": 7}
  ```

### 3.5 The Task Tracker service, observable
- **Goal:** everything together: JSON logs, request ID, recovery, metrics, and `pprof` on a separate loopback admin port.
- **Run:**
  ```bash
  LOG_STABLE=1 ADDR=127.0.0.1:8080 ADMIN_ADDR=127.0.0.1:6060 go run ./modules/10-logging-and-diagnostics/examples/05_task_service
  ```
  Ctrl-C triggers a graceful shutdown of both servers. `LOG_STABLE=1` drops timestamps and durations from log lines.
- **Code:** [examples/05_task_service/main.go](examples/05_task_service/main.go)
- **Walkthrough:** `app.wrap` = request ID -> access log -> recover. The admin mux registers `pprof` handlers explicitly (importing `net/http/pprof` also registers them on `http.DefaultServeMux`, which this program never serves). The admin server has no `WriteTimeout` because a CPU profile takes 30 s.
- **Expected output** (in another terminal; generated request IDs are random, so only the fixed one is shown):
  ```
  $ curl -s -D - -o /dev/null -XPOST localhost:8080/tasks -H 'X-Request-ID: demo-1' -d '{"title":"Write docs"}' | grep -i -E 'HTTP|x-request'
  HTTP/1.1 201 Created
  X-Request-Id: demo-1
  $ curl -s localhost:8080/panic
  {"error":{"code":"internal","message":"internal server error"}}
  $ curl -s localhost:8080/readyz
  ready
  $ curl -s localhost:8080/metrics
  requests_total 4
  panics_total 1
  $ curl -s -o /dev/null -w '%{http_code}\n' localhost:6060/debug/pprof/     # admin port
  200
  $ curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/debug/pprof/     # public port
  404
  ```
  Server log lines for the first request:
  ```
  {"level":"INFO","msg":"task created","id":1,"request_id":"demo-1"}
  {"level":"INFO","msg":"request","method":"POST","path":"/tasks","status":201,"request_id":"demo-1"}
  ```
  Profile it: `go tool pprof http://127.0.0.1:6060/debug/pprof/heap`.

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `logger.Info("x")` in helpers loses the request ID | Without a `ctx` the context handler cannot see the value | Use `InfoContext(ctx, ...)` everywhere you have a context |
| Custom `slog.Handler` wrapper forgets `WithAttrs`/`WithGroup` | Embedded methods return the *inner* handler, dropping your decoration | Re-wrap in both methods (3.2) |
| Odd number of key/value args (`!BADKEY`) | `Info("m", "k")` compiles | `go vet` catches it; prefer `slog.Int(...)` + `LogAttrs` on hot paths |
| Logging secrets (`password`, tokens, whole request bodies, query strings) | `%+v` of a struct or `r.URL.String()` includes them | `LogValuer`/redacting type, `ReplaceAttr`, log `r.URL.Path` only |
| Status code is always 200 in the access log | The wrapper never saw `WriteHeader`, or the handler wrote only a body | Wrapper records the first `WriteHeader`, defaults to 200 on first `Write` |
| Wrapper breaks `Flush`/`Hijack`/`SetWriteDeadline` | Embedding `http.ResponseWriter` hides the optional interfaces | Implement `Unwrap() http.ResponseWriter` and use `http.ResponseController` |
| Recover middleware writes a body after the response started | Handler already sent headers; the 500 is impossible | Track whether the response started; just log |
| Swallowing `http.ErrAbortHandler` | `recover()` catches every panic value | Re-panic that one value |
| `panic` in a goroutine you started | `recover` only works in the panicking goroutine; the process dies | `defer recover()` inside each goroutine you spawn |
| `import _ "net/http/pprof"` plus serving `DefaultServeMux` publicly | The import registers `/debug/pprof/*` on the default mux | Own mux per server; pprof on a loopback/admin port, behind auth if remote |
| Request IDs copied unchecked from headers | Newlines/long values forge or flood logs | Accept 1-64 visible ASCII chars, else generate (exercise 2) |

## 5. Coming From Other Languages
- **Java SLF4J/Logback, Python `logging`, Node `pino`:** `slog` has the same ideas (levels, handlers, structured fields) in the standard library. MDC/contextvars are replaced by explicit `context.Context` plus a decorating handler; nothing is thread-local.
- **Express/ASP.NET middleware:** a Go middleware is just `func(http.Handler) http.Handler`; there is no `next()` callback, you call `next.ServeHTTP`. Exception filters become a `recover` middleware.
- **Spring Actuator / ASP.NET HealthChecks:** you write the three handlers yourself (a few lines); `pprof` replaces JFR/dotTrace-style profilers and ships in the standard library.
- **Rust `tracing`:** spans carry context automatically; in Go the context value plus handler decoration gives the same correlation without spans (tracing is out of scope here).

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Logger factory | Easy | handlers, `LevelVar`, `ReplaceAttr` redaction | `go test ./modules/10-logging-and-diagnostics/exercises/ex1_slog_setup/` |
| 2 | Request-ID + access-log middleware | Medium | writer wrapper, context, log levels, `Unwrap` | `go test ./modules/10-logging-and-diagnostics/exercises/ex2_request_logging/` |
| 3 | Recover + metrics | Stretch | `recover` rules, started responses, atomics, `ErrAbortHandler` | `go test ./modules/10-logging-and-diagnostics/exercises/ex3_recover_metrics/` |

**Exercise 1: logger factory**
- **Task:** implement `New` in [exercises/ex1_slog_setup/logsetup.go](exercises/ex1_slog_setup/logsetup.go).
- **Acceptance:** JSON by default, text on request; `service` attribute; case-insensitive level names with `ErrBadOption` on bad input; the returned `LevelVar` changes the live level; `password`/`token` keys are redacted even inside groups.
- <details><summary>Hint</summary>`HandlerOptions{Level: &levelVar, ReplaceAttr: ...}`; `ReplaceAttr` receives leaf attributes with their group path, so a key check is enough.</details>

**Exercise 2: request-ID and access-log middleware**
- **Task:** implement `RequestID` and `Middleware` in [exercises/ex2_request_logging/reqlog.go](exercises/ex2_request_logging/reqlog.go).
- **Acceptance:** one `request` record per request with all fields; hostile IDs replaced; level by status class; implicit `200`; byte count over multiple writes; no query string in the log; `Flush` still works through the wrapper.
- <details><summary>Hint</summary>Wrap `ResponseWriter` in a struct with `status` and `bytes`, record the first `WriteHeader`, and add `Unwrap() http.ResponseWriter`.</details>

**Exercise 3: recover + metrics**
- **Task:** implement `Metrics.Middleware`, `Metrics.Handler` and `Recover` in [exercises/ex3_recover_metrics/guard.go](exercises/ex3_recover_metrics/guard.go).
- **Acceptance:** panic becomes the JSON 500 envelope without leaking the value; stack and panic logged; no error body appended to a started response; `http.ErrAbortHandler` re-panicked; `in_flight` returns to 0 after a panic; counters correct under 200 concurrent requests.
- <details><summary>Hint</summary>`defer m.inFlight.Add(-1)` runs even while panicking. For "response started", wrap the writer and remember whether `Write` or `WriteHeader` was called.</details>

## 7. Recap
- Use `slog` with key-value attributes; JSON in production, text locally; one logger built in `main`.
- Carry the request ID in the `context` and log with `*Context` methods; decorate the handler once.
- Middleware order matters: request ID -> access log -> recover -> handler, so the log sees recovered 500s.
- Liveness is "process is up", readiness is "send traffic"; flip readiness off before shutdown.
- Never log secrets; keep `pprof` and `expvar` off the public port.

## 8. Self-check Questions
1. Why does a log line written with `logger.Info(...)` inside a repository method lack the request ID, and what is the smallest change that fixes it?
2. If `Recover` were the *outermost* middleware and the access log the inner one, what would the access log record for a panicking handler?
3. Why can `recover` not turn a panic into a 500 after the handler has already written part of the response?
4. What is the difference between a liveness and a readiness probe, and what goes wrong if liveness checks the database?

## 9. Further Reading
- [`log/slog` package](https://pkg.go.dev/log/slog) and the Go blog post [Structured Logging with slog](https://go.dev/blog/slog)
- [`net/http/pprof`](https://pkg.go.dev/net/http/pprof) and [Profiling Go Programs](https://go.dev/blog/pprof)
- [`expvar`](https://pkg.go.dev/expvar), [`http.ResponseController`](https://pkg.go.dev/net/http#ResponseController)

---
**Next:** [11 Authentication and Security Basics](../11-auth-and-security-basics/README.md)
