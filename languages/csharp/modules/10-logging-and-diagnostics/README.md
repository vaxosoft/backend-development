# 10. Logging and Diagnostics in C#

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 09

## Learning Outcomes
After this module you can:
- [ ] Write structured log entries with message templates, levels and scopes, and filter them per category.
- [ ] Add a request-logging middleware with correlation ids.
- [ ] Turn unhandled exceptions into safe `application/problem+json` responses while logging the details.
- [ ] Expose health (liveness/readiness) and basic metrics endpoints.
- [ ] Record counters, histograms and spans with `System.Diagnostics`, and know the profiling tools.

## 1. Concept Overview
Logs are for humans and machines investigating *what happened to one request*; metrics answer *how is the system doing overall*; health checks tell the platform *whether to send traffic or restart me*. **Structured** logging keeps the values as named fields (`TaskId=5`) next to the message, so a log system can filter and aggregate them. A **correlation id** (request id) in every line of a request lets you reassemble its story. Errors are logged with full detail on the server and answered with a generic body: the client never sees stack traces, connection strings or SQL.

## 2. How C# Approaches It
| Piece | Role |
|-------|------|
| `ILogger<T>` (`Microsoft.Extensions.Logging`) | Injected logger, category = `T`'s full name. Levels: Trace, Debug, Information, Warning, Error, Critical |
| Message templates | `logger.LogInformation("Task {TaskId} created", id)`: placeholders become structured properties; `[LoggerMessage]` source generation is the fast path |
| Scopes | `using (logger.BeginScope("request {RequestId}", id))` adds context to every line inside |
| Providers and formatters | Console (simple or **JSON** formatter), Debug, EventSource; OpenTelemetry exporters plug in the same way |
| Filtering | `appsettings.json` `Logging:LogLevel:Category`, or `AddFilter(...)` |
| Exception handling | `UseExceptionHandler` + `IExceptionHandler` + `AddProblemDetails` (RFC 9457) |
| `HealthChecks` | `AddHealthChecks()`, `MapHealthChecks(path, options)` with predicates/tags for liveness vs readiness |
| `System.Diagnostics.Metrics`, `Activity` | `Meter`/`Counter`/`Histogram` and `ActivitySource` (tracing); consumed by `dotnet-counters`, `dotnet-trace`, OpenTelemetry |

Idiomatic: templates with named placeholders, scopes for context, middleware for cross-cutting logging, `ILogger<T>` through DI. Non-idiomatic: `$"..."` interpolation in log calls, `Console.WriteLine`, `catch (Exception) { }`, logging inside tight loops at Information level. No third-party packages here (Serilog/OpenTelemetry are mentioned, not used).

## 3. Guided Examples
Run from `languages/csharp/`: `dotnet run --project modules/10-logging-and-diagnostics/examples -- <name>`. Examples 1-4 capture or print everything themselves and exit; example 5 is a server.

### 3.1 Levels, categories and message templates
- **Goal:** see what a log call really carries.
- **Run:** `dotnet run --project modules/10-logging-and-diagnostics/examples -- 01-logger-basics`
- **Code:** [examples/01_logger_basics.cs](examples/01_logger_basics.cs) (uses the in-memory [CaptureLoggerProvider](examples/Support/CaptureLoggerProvider.cs) to print the structured state)
- **Walkthrough:** the minimum level drops `Debug`; `AddFilter("M10.Noisy", Warning)` overrides one category. With a template the state holds `TaskId` and `Owner` plus `{OriginalFormat}`; the interpolated call (`$"Task {taskId}..."`) produces the same text but **no properties**. `[LoggerMessage]` gives a compile-checked, allocation-light version.
- **Expected output:**
  ```
  Information M10.Orders Task 5 assigned to ada
              properties: TaskId=5, Owner=ada, {OriginalFormat}=Task {TaskId} assigned to {Owner}
  Warning     M10.Orders Task 5 is overdue by 3 days
              properties: TaskId=5, Days=3, {OriginalFormat}=Task {TaskId} is overdue by {Days} days
  Warning     M10.Noisy  kept: warnings pass the M10.Noisy rule
              properties: {OriginalFormat}=kept: warnings pass the M10.Noisy rule
  Information M10.Orders Task 6 assigned to grace
              properties: {OriginalFormat}=Task 6 assigned to grace
  Error       M10.Orders Could not save task 5
              properties: TaskId=5, {OriginalFormat}=Could not save task {TaskId}
              exception: InvalidOperationException: database is read-only
  Information M10.Orders Task 7 assigned to linus (generated)
              properties: TaskId=7, Owner=linus, {OriginalFormat}=Task {TaskId} assigned to {Owner} (generated)
  orders.IsEnabled(Debug)=False noisy.IsEnabled(Information)=False
  ```

### 3.2 Scopes and the JSON console formatter
- **Goal:** machine-readable lines that carry request context automatically.
- **Run:** `dotnet run --project modules/10-logging-and-diagnostics/examples -- 02-scopes-and-json`
- **Code:** [examples/02_scopes_and_json.cs](examples/02_scopes_and_json.cs)
  ```csharp
  builder.AddJsonConsole(o => { o.IncludeScopes = true; o.JsonWriterOptions = new() { Indented = false }; });
  using (logger.BeginScope("request {RequestId} by user {UserId}", "req-42", 7)) { logger.LogInformation("Task {TaskId} created", 5); }
  ```
- **Walkthrough:** each line has `State` (the call's placeholders) and `Scopes` (every enclosing scope, innermost last). The console logger writes on a background thread; disposing the factory flushes it. A timestamp is added with `o.TimestampFormat`; it is left off here for reproducible output.
- **Expected output:**
  ```
  JSON lines written by the console logger:
  {"EventId":0,"LogLevel":"Information","Category":"TaskService","Message":"Service started","State":{"{OriginalFormat}":"Service started"},"Scopes":[]}
  {"EventId":0,"LogLevel":"Information","Category":"TaskService","Message":"Task 5 created","State":{"TaskId":5,"{OriginalFormat}":"Task {TaskId} created"},"Scopes":[{"Message":"request req-42 by user 7","RequestId":"req-42","UserId":7,"{OriginalFormat}":"request {RequestId} by user {UserId}"}]}
  {"EventId":0,"LogLevel":"Warning","Category":"TaskService","Message":"Task 6 skipped: duplicate title","State":{"TaskId":6,"Reason":"duplicate title","{OriginalFormat}":"Task {TaskId} skipped: {Reason}"},"Scopes":[{"Message":"request req-42 by user 7","RequestId":"req-42","UserId":7,"{OriginalFormat}":"request {RequestId} by user {UserId}"},{"Message":"importing 3 tasks","Count":3,"{OriginalFormat}":"importing {Count} tasks"}]}
  {"EventId":0,"LogLevel":"Information","Category":"TaskService","Message":"Back outside every scope","State":{"{OriginalFormat}":"Back outside every scope"},"Scopes":[]}
  ```

### 3.3 Exception-handling middleware and problem details
- **Goal:** a safe error contract: log everything, leak nothing.
- **Run:** `dotnet run --project modules/10-logging-and-diagnostics/examples -- 03-exception-handling`
- **Code:** [examples/03_exception_handling.cs](examples/03_exception_handling.cs)
- **Walkthrough:** `UseExceptionHandler()` catches what the endpoint throws and calls `ProblemExceptionHandler`: a domain exception (`TaskNotFoundException`) maps to `404` with a detail; anything else maps to a generic `500` whose real message (here a file path) appears **only** in the server log. `UseStatusCodePages()` gives bodiless statuses such as an unknown route a problem body too. The random `traceId` is removed to keep the output stable; the example echoes the caller's `X-Request-ID` as `request_id`. The C# track uses problem+json (RFC 9457) instead of the `{"error":{...}}` envelope of the Go/Rust tracks; status codes are identical.
- **Expected output:**
  ```
  GET /tasks/1 -> 200 application/json; charset=utf-8
    {"id":1,"title":"Write docs"}
  GET /tasks/9 -> 404 application/problem+json
    {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.5","title":"Task not found","status":404,"detail":"task 9 does not exist","request_id":"req-2"}
  GET /boom -> 500 application/problem+json
    {"type":"https://tools.ietf.org/html/rfc9110#section-15.6.1","title":"An unexpected error occurred","status":500,"request_id":"req-3"}
  GET /no-such-route -> 404 application/problem+json
    {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.5","title":"Not Found","status":404,"request_id":"req-4"}
  server log:
    Warning: Request failed with 404: task 9 does not exist
    Error: Unhandled exception for GET /boom [InvalidOperationException: disk quota exceeded writing /var/lib/tt/tasks.db]
  ```

### 3.4 Metrics and activities
- **Goal:** instrument code once; let tools subscribe.
- **Run:** `dotnet run --project modules/10-logging-and-diagnostics/examples -- 04-metrics-and-activities`
- **Code:** [examples/04_metrics_and_activities.cs](examples/04_metrics_and_activities.cs)
- **Walkthrough:** `Meter("TaskTracker")` owns a `Counter<long>`, a `Histogram<double>` and an `ObservableGauge`; tags are dimensions. A `MeterListener` plays the role of `dotnet-counters monitor TaskTracker` or an OpenTelemetry exporter. `ActivitySource` spans nest through `Activity.Current`; with no listener `StartActivity` returns `null` (hence `request?.SetTag`).
- **Expected output:**
  ```
  measurements:
    tasks_created += 1 [status=todo]
    tasks_created += 2 [status=done]
    request_duration recorded 12.5 ms [route=/tasks]
    queue_length = 4
  spans:
    stopped 'SaveTask' parent=POST /tasks tags=[task.id=5]
    stopped 'POST /tasks' parent=none tags=[http.route=/tasks]
  stopwatch elapsed is non-negative: True
  ```

### 3.5 Task Tracker diagnostics service
- **Goal:** the pieces together: request-id + access log middleware, exception handler, liveness/readiness, `/metrics`, graceful shutdown.
- **Run:** `ADDR=127.0.0.1:8080 dotnet run --project modules/10-logging-and-diagnostics/examples -- 05-task-tracker-diagnostics` (default `127.0.0.1:8080`; Ctrl-C or SIGTERM stops it cleanly)
- **Code:** [examples/05_task_tracker_diagnostics.cs](examples/05_task_tracker_diagnostics.cs) (reuses `ProblemExceptionHandler` from example 3)
- **Walkthrough:** `RequestLoggingMiddleware` is outermost, so it sees the final status even when the exception handler rewrites a throw into `500`. It reuses a valid incoming `X-Request-ID` (length-limited, safe characters) or creates one, sets the header in `OnStarting` (the exception handler clears headers), opens a scope, and logs one JSON line. `/healthz` runs no checks (liveness); `/readyz` runs the `ready` check, which is `Unhealthy` until `ApplicationStarted` fires and again after `ApplicationStopping` (readiness). `POST /admin/ready/false` is a demo switch to see the `503`.
- **Expected output** (a second terminal; `-i` headers trimmed to the relevant ones):
  ```
  $ curl -si -H 'X-Request-ID: demo-1' localhost:8080/readyz
  HTTP/1.1 200 OK
  Content-Type: text/plain
  X-Request-ID: demo-1

  Healthy

  $ curl -si -H 'X-Request-ID: demo-1' localhost:8080/tasks/9
  HTTP/1.1 404 Not Found
  Content-Type: application/problem+json
  X-Request-ID: demo-1

  {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.5","title":"Task not found","status":404,"detail":"task 9 does not exist","request_id":"demo-1"}

  $ curl -si -H 'X-Request-ID: demo-1' localhost:8080/boom
  HTTP/1.1 500 Internal Server Error
  Content-Type: application/problem+json
  X-Request-ID: demo-1

  {"type":"https://tools.ietf.org/html/rfc9110#section-15.6.1","title":"An unexpected error occurred","status":500,"request_id":"demo-1"}

  $ curl -s localhost:8080/metrics
  requests_total 3
  requests_by_status{status="200"} 1
  requests_by_status{status="404"} 1
  requests_by_status{status="500"} 1

  $ curl -s -X POST localhost:8080/admin/ready/false -o /dev/null -w '%{http_code}\n'; curl -si localhost:8080/readyz | head -1
  204
  HTTP/1.1 503 Service Unavailable
  ```
  The server's stdout holds one JSON line per request, e.g. (duration varies):
  `{"LogLevel":"Information","Category":"M10.Examples.RequestLoggingMiddleware","Message":"HTTP GET /tasks/9 responded 404 in 104.6 ms","State":{"Method":"GET","Path":"/tasks/9","StatusCode":404,"DurationMs":104.573,...},"Scopes":[...{"Message":"request demo-1","RequestId":"demo-1",...}]}` and, for `/boom`, an `Error` line with the full `Exception` text.

**Profiling and debugging basics:** `dotnet-counters monitor --process-id <pid> TaskTracker` (live metrics, works with example 4's meter), `dotnet-trace collect` (CPU samples; open in Visual Studio/PerfView/Speedscope), `dotnet-dump collect` for hangs, and the IDE debugger with conditional breakpoints. `Stopwatch.GetTimestamp()`/`GetElapsedTime` for quick measurements.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `logger.LogInformation($"Task {id} created")` | Interpolation is natural C#; it formats eagerly and loses the `TaskId` property | Template + arguments: `LogInformation("Task {TaskId} created", id)` (analyzer CA2254 flags it) |
| Placeholders in the wrong order | Arguments bind **by position**, names are only labels | Keep order identical; `[LoggerMessage]` checks the count |
| Logging PII, tokens, passwords, whole request bodies or query strings | "Helpful for debugging" | Log ids not values, redact (`token=***`), never log `Authorization` or request bodies |
| `catch (Exception ex) { logger.LogError(ex.Message); }` | Loses the stack trace and type | `logger.LogError(ex, "Could not save task {TaskId}", id)`; rethrow or handle once, not at every layer |
| Returning `ex.ToString()`/`ex.Message` in the response | Convenient for clients | Generic problem+json for 5xx; details only in the log (`UseExceptionHandler`, not the developer exception page, outside Development) |
| Exception-handler middleware registered after the middleware that throws | Order defines wrapping | Register `UseExceptionHandler()` first (after your logging middleware if you want it to see the final status) |
| Trusting an incoming `X-Request-ID` | Header values can contain newlines or megabytes | Validate (length, character set) or generate your own |
| One health endpoint for both liveness and readiness | Restarting a pod because a database is down turns an outage into a crash loop | `/healthz` = process only, `/readyz` = dependencies |
| `Console.WriteLine` for logs, or logging at `Information` in hot loops | Bypasses levels/filters; volume costs money | `ILogger`, `IsEnabled`/`[LoggerMessage]`, `Debug` for chatty details |
| Unbounded metric tags (user id, full URL) | Looks like useful detail | Low-cardinality tags (route template, status class) |

## 5. Coming From Other Languages
- **Java:** SLF4J `log.info("x {}", v)` + MDC ~ templates + `BeginScope`; Logback JSON encoders ~ the JSON console formatter; Micrometer ~ `System.Diagnostics.Metrics`; Spring Actuator `/health` ~ health checks.
- **Go:** `slog` with attributes ~ structured templates; `slog.With(...)` ~ scopes; `recover()` middleware ~ `UseExceptionHandler`; `expvar`/Prometheus client ~ `Meter`.
- **TypeScript/JS:** `pino`/`winston` child loggers ~ scopes; Express error middleware ~ `IExceptionHandler`; `prom-client` ~ `Meter`.
- **Python:** `logging` with `extra=`/`structlog` ~ templates and properties; `contextvars` correlation ids ~ scopes; middleware for exceptions ~ `UseExceptionHandler`.

## 6. Exercises
Run one exercise at a time from `languages/csharp/`. Solutions are in [solutions/](solutions/). Tests drive middleware directly with `DefaultHttpContext`, a `ListLogger<T>` and `FakeTimeProvider` (see [TestSupport.cs](exercises/TestSupport.cs)).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Log safety | Easy | Correlation-id validation, redacting secrets from logged query strings | `dotnet test modules/10-logging-and-diagnostics/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Request logging middleware | Medium | Structured access log, scopes, levels by status, `TimeProvider`, rethrow | `dotnet test modules/10-logging-and-diagnostics/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Problem exception handler | Stretch | `IExceptionHandler`, problem+json, no leaks, log level per outcome | `dotnet test modules/10-logging-and-diagnostics/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: [Ex1_LogSafety.cs](exercises/Ex1_LogSafety.cs)
- **Task:** `ResolveCorrelationId` (trust only short, safe ids) and `RedactQuery` (mask `token`, `password`, ... values, keep everything else byte-for-byte).
- **Acceptance:** `Ex1_LogSafetyTests`.
- **Hint:** <details><summary>Show</summary>`char.IsAsciiLetterOrDigit` for the id; split the query on `&`, then on the first `=`, and compare the name with a case-insensitive `HashSet`.</details>

### Exercise 2: [Ex2_RequestLogging.cs](exercises/Ex2_RequestLogging.cs)
- **Task:** `RequestLoggingMiddleware.InvokeAsync`: request id in/out, `request {RequestId}` scope, one entry with the exact template, level by status, exception -> `500` entry then rethrow.
- **Acceptance:** `Ex2_RequestLoggingTests` (properties, elapsed time from `FakeTimeProvider`, scope on downstream logs and closed afterwards, no query string in the log).
- **Hint:** <details><summary>Show</summary>Use a `const string Template` and `logger.Log(level, Template, ...)`; `time.GetTimestamp()` and `time.GetElapsedTime(start)`; `try`/`catch (Exception ex) { ...; throw; }`.</details>

### Exercise 3: [Ex3_ProblemExceptionHandler.cs](exercises/Ex3_ProblemExceptionHandler.cs)
- **Task:** implement `ProblemExceptionHandler.TryHandleAsync` for not-found, validation, client-abort and unexpected exceptions.
- **Acceptance:** `Ex3_ProblemExceptionHandlerTests` (status, content type, body members, no leaked message, log level and exception attachment, `request_id`, `499` without body).
- **Hint:** <details><summary>Show</summary>Build a `ProblemDetails`, put extras in `Extensions`, write with `Response.WriteAsJsonAsync(problem, options: null, contentType: "application/problem+json", ct)`.</details>

## 7. Recap
- Use message templates (`{Name}` placeholders) and exceptions as the first argument: logs stay structured and searchable.
- Put request context in scopes and a correlation id on every request and response.
- One exception-handling middleware owns the error contract: detailed log, generic problem+json.
- Liveness says "restart me", readiness says "send me traffic"; metrics say "how am I doing".
- When to reach for what: `ILogger` for events, `Meter` for numbers over time, `Activity` for causality across calls, health checks for the platform.

## 8. Self-check Questions
1. Two log calls produce the same text, one with a template and one with interpolation. What can you do with the first one in a log platform that you cannot do with the second?
2. Why does the access-log middleware sit outside the exception handler, and what status would it log if it were inside?
3. When should `/readyz` fail while `/healthz` stays green? What goes wrong if both are the same endpoint?
4. Which pieces of a failing request go into the log, and which into the response body?
5. Why is `user_id` a poor metric tag but fine as a log property?

## 9. Further Reading
- [Logging in .NET](https://learn.microsoft.com/dotnet/core/extensions/logging) and [compile-time logging source generation](https://learn.microsoft.com/dotnet/core/extensions/logger-message-generator)
- [Handle errors in ASP.NET Core APIs](https://learn.microsoft.com/aspnet/core/fundamentals/error-handling) and [Health checks](https://learn.microsoft.com/aspnet/core/host-and-deploy/health-checks)
- [.NET observability with OpenTelemetry](https://learn.microsoft.com/dotnet/core/diagnostics/observability-with-otel) and [dotnet-counters](https://learn.microsoft.com/dotnet/core/diagnostics/dotnet-counters)

---
**Next:** [11 Authentication and Security Basics](../11-auth-and-security-basics/README.md)
