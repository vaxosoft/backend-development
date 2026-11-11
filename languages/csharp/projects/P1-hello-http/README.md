# P1. Hello HTTP

> Modules exercised: [01 Language fundamentals](../../modules/01-language-fundamentals/README.md), [02 HTTP server](../../modules/02-http-server/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md)

## Goal

Build a tiny ASP.NET Core Minimal API with the production basics around it: a small middleware pipeline (request id, request logging, exception handling), configured server timeouts, and a **graceful shutdown** that lets an in-flight request finish while refusing new connections.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | `200` `{"status":"ok"}` (`application/json`) |
| `GET /hello` | `200` `text/plain` `Hello, world!` |
| `GET /hello/{name}` | `200` `text/plain` `Hello, {name}!` (percent-decoded) |
| `GET /slow?ms=N` | waits `N` ms (missing = `0`, values above **2000 are clamped to 2000**) using the injected `TimeProvider`, then `200` `done`. A non-integer or negative `ms` is `400` problem+json |
| `GET /panic` | the handler throws; the response is `500` `application/problem+json` with no exception details, and the server keeps serving |

Middleware, in this order (outermost first):

1. **Request id**: echo `X-Request-ID` when it is at most 64 chars of `[A-Za-z0-9._-]`, otherwise generate one. Store it in `HttpContext.TraceIdentifier` and put it on every response, including errors.
2. **Request logging**: one `RequestLogEntry` (method, path without query, status, duration, request id) per request, written to the injected `IRequestLogSink`.
3. **Exception handling**: log the exception, answer `500` problem+json (`status`, `title`, `request_id`), never leak the exception message or stack. Do not try to answer requests the client already aborted.

Server configuration (`HelloApp.ConfigureTimeouts`): `RequestHeadersTimeout` <= 10 s, `KeepAliveTimeout` <= 60 s, `HostOptions.ShutdownTimeout` <= 15 s.

Graceful shutdown (`HelloApp.RunAsync(addr, shutdown, sink, onListening)`): when `shutdown` is cancelled (or on SIGINT/SIGTERM) the listener closes immediately, in-flight requests complete, then the task finishes. `onListening` receives the real URL (use port `0` in tests).

> The error body is `application/problem+json` (RFC 9457), the idiomatic ASP.NET Core equivalent of the track's JSON error envelope.

## Acceptance criteria -> tests

| Criterion | Tests (`tests/`) |
|-----------|------------------|
| health | `Healthz_ReturnsOkJson` |
| hello | `Hello_WithName_GreetsTheName`, `Hello_WithoutName_GreetsTheWorld`, `Hello_IsPlainText`, `Hello_DecodesPercentEncodedNames`, `UnknownRoute_Returns404` |
| slow | `Slow_WaitsTheRequestedTimeThenSaysDone`, `Slow_ClampsTheDelayTo2000Ms`, `Slow_WithInvalidMs_Returns400` |
| panic + recovery | `Panic_Returns500ProblemJson`, `Panic_DoesNotLeakExceptionDetails`, `Panic_ServerKeepsServing` |
| request id | `RequestId_IsEchoedWhenProvided`, `RequestId_IsGeneratedWhenMissingAndDiffersPerRequest`, `RequestId_IsReplacedWhenIncomingValueIsUnreasonable`, `RequestId_IsPresentOnErrorResponses` |
| request logging | `Logging_WritesOneEntryPerRequestWithMethodPathStatus`, `Logging_RecordsThePanicAs500` |
| timeouts | `Server_TimeoutsAreConfigured` |
| graceful shutdown | `Shutdown_LetsAnInFlightSlowRequestFinish`, `Shutdown_RefusesNewConnectionsAfterwards`, `Shutdown_ReturnsPromptlyWhenIdle` |

## Run

From `languages/csharp/`:

```bash
export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
ADDR=127.0.0.1:8080 dotnet run --project projects/P1-hello-http/starter   # ADDR defaults to 127.0.0.1:8080
curl -i localhost:8080/hello/Ada
curl -i -H 'X-Request-ID: demo-1' localhost:8080/panic
```

The reference solution answers:

```text
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
X-Request-ID: 2f8c...            (generated; echoed when you send one)

Hello, Ada!

HTTP/1.1 500 Internal Server Error
Content-Type: application/problem+json
X-Request-ID: demo-1

{"type":"https://tools.ietf.org/html/rfc9110#section-15.6.1","title":"An unexpected error occurred","status":500,"request_id":"demo-1"}
```

Stop with Ctrl-C (or `kill <pid>`): in-flight requests are finished first.

## Test

```bash
dotnet test projects/P1-hello-http/tests
```

The starter compiles and runs, but most tests fail until you replace the `TODO`s in `Endpoints.cs`, `Middleware.cs` and `HelloApp.cs`. Everything else (types, wiring, `Program.cs`) is given. The tests use `WebApplicationFactory<Program>` with a capturing `IRequestLogSink` and a `TimeProvider` that records delays (so nothing really sleeps); only the three shutdown tests start a real Kestrel on `127.0.0.1:0`.

## Steps and hints

1. Endpoints first (`Endpoints.cs`): `hello`, `slow`, `panic`.
2. `RequestIdMiddleware`, then `RequestLoggingMiddleware`, then `ExceptionHandlingMiddleware`.
3. `ConfigureTimeouts`, then the shutdown token in `RunAsync`.

<details>
<summary>Hints</summary>

- `Results.Text(...)` gives `text/plain`. `{name?}` makes a route parameter optional.
- Bind `ms` as `string?` and parse it yourself: a failed *automatic* binding can surface as an exception (`ThrowOnBadRequest`) instead of a response.
- `await Task.Delay(delay, timeProvider, ct)` uses the injected clock; pass the request's `CancellationToken`.
- Read the status in a `finally` block after `await next(context)`, otherwise exceptions skip your log line. `Stopwatch.GetTimestamp()` + `Stopwatch.GetElapsedTime(start)` measure durations without allocating.
- `Results.Problem(...).ExecuteAsync(context)` writes problem+json from a middleware. Call `context.Response.Clear()` first and check `Response.HasStarted`.
- Kestrel: `builder.WebHost.ConfigureKestrel(o => o.Limits.RequestHeadersTimeout = ...)`; host: `builder.Services.Configure<HostOptions>(o => o.ShutdownTimeout = ...)`.
- `app.WaitForShutdownAsync(token)` returns on cancellation, SIGINT or SIGTERM and stops the host gracefully.

</details>

## Stretch goals (not tested)

- Add `GET /readyz` that returns `503` once shutdown has started (`IHostApplicationLifetime.ApplicationStopping`).
- Make the log line JSON (`AddJsonConsole`) and add the request id as a logging scope.
- Cancel `/slow` when the client disconnects and log those as `499`.
- Add `app.UseExceptionHandler` + `AddProblemDetails()` and compare with your own middleware.

## Reference solution

`solution/` contains only the files that differ from `starter/` (`HelloApp.cs`, `Endpoints.cs`, `Middleware.cs`). Try the exercise first; overlay it with `cp -r projects/P1-hello-http/solution/. projects/P1-hello-http/starter/` to check your work (this overwrites your changes).
