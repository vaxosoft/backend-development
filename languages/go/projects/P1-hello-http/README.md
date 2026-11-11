# P1 - Hello HTTP

> **Modules exercised:** [01 Language Fundamentals](../../modules/01-language-fundamentals/README.md), [02 HTTP Server](../../modules/02-http-server/README.md), [10 Logging and Diagnostics](../../modules/10-logging-and-diagnostics/README.md)
> **Time:** 1-2 hours · **Difficulty:** Beginner

## Goal

Build a small HTTP server with routing, three middlewares (request ID, request logging, panic recovery), explicit server timeouts and a **graceful shutdown** that lets in-flight requests finish.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | `200`, `application/json`, `{"status":"ok"}` |
| `GET /hello` | `200`, `text/plain`, `Hello, world!` |
| `GET /hello/{name}` | `200`, `text/plain`, `Hello, {name}!` (percent-encoding decoded) |
| `GET /slow?ms=N` | sleeps `N` ms (clamped to 2000; missing = 0) then `200` `done`; stops early if the client disconnects; invalid `ms` (not a non-negative integer) -> `400` `bad_request` envelope |
| `GET /panic` | the handler panics; the client gets `500` with `{"error":{"code":"internal","message":"internal server error"}}` (the panic value is never leaked) and the server keeps serving |

Any other method on these paths gets `405` with an `Allow` header (the `ServeMux` does this for you).

**Middleware** (outermost first: `RequestID`, `Logging`, `Recover`):

- `X-Request-ID`: echo the incoming header if it is 1-64 printable ASCII characters, otherwise generate a random ID. It is set on every response (also `500`s) and stored in the request context.
- Logging: exactly one line per request to the injected `io.Writer`, written after the handler returns, with `request_id`, `method`, `path`, `status` and `duration` (use `log/slog`). A panicking request is logged with `status=500`.
- Recovery: re-panic `http.ErrAbortHandler`, otherwise answer with the `500` envelope.

**Server**: `NewServer` sets `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout` (longer than the 2 s `/slow` maximum) and `IdleTimeout`. `Run(ctx, ln, h, drain)` serves until `ctx` is cancelled, then closes the listener immediately, waits up to `drain` for in-flight requests and returns `nil`; if the drain time is exceeded it closes the remaining connections and returns an error wrapping `context.DeadlineExceeded`.

## Layout

```
projects/P1-hello-http/
├── starter/            package starter (+ cmd/server/main.go, complete)
│   ├── app.go          New(Config) http.Handler  - wiring, complete
│   ├── handlers.go     TODO  WriteError (given), Healthz, Hello, SlowDelay, Slow, Panic
│   ├── middleware.go   TODO  RequestID, Logging, Recover (Chain and statusRecorder are given)
│   └── server.go       TODO  NewServer, Run
├── _solution/           files that differ from starter/ (overlay)
└── tests/              black-box tests over real HTTP (never overlaid)
```

## Acceptance criteria

| Criterion | Tests |
|-----------|-------|
| Health endpoint | `TestHealthzReturnsStatusOKJSON` |
| Greeting, encoding, default | `TestHelloGreetsName`, `TestHelloDecodesPercentEncodedName`, `TestHelloWithoutNameGreetsWorld` |
| Method routing | `TestWrongMethodIsRejectedWithAllowHeader` |
| `/slow` waits, clamps, validates | `TestSlowWaitsThenAnswersDone`, `TestSlowDelayParsesAndClamps`, `TestSlowRejectsInvalidMsWithErrorEnvelope` |
| Panic recovery | `TestPanicIsAnswered500WithEnvelopeAndNoLeak`, `TestServerKeepsServingAfterPanic` |
| Request ID | `TestRequestIDIsGeneratedWhenAbsent`, `TestRequestIDIsUniquePerRequest`, `TestRequestIDIsEchoed`, `TestOverlongRequestIDIsReplaced`, `TestRequestIDIsPresentOnPanicResponses` |
| Request log | `TestLoggingWritesOneLinePerRequestWithFields`, `TestLoggingWritesALinePerRequest`, `TestLoggingRecordsStatus500ForPanics` |
| Timeouts | `TestServerTimeoutsAreConfigured` |
| Graceful shutdown | `TestShutdownLetsInFlightRequestFinish`, `TestShutdownRefusesNewConnections`, `TestShutdownGivesUpAfterDrainTimeout` |

## How to run

From `languages/go/`:

```bash
ADDR=127.0.0.1:8080 go run ./projects/P1-hello-http/starter/cmd/server   # ADDR defaults to 127.0.0.1:8080
go test ./projects/P1-hello-http/tests/                                  # fails until you are done
go test -race -count=1 ./projects/P1-hello-http/tests/
```

Try it (Ctrl-C or `kill -TERM <pid>` shuts down gracefully):

```bash
curl -s -w ' [%{http_code}]\n' localhost:8080/healthz          # {"status":"ok"} [200]
curl -s -w ' [%{http_code}]\n' localhost:8080/hello/Gopher     # Hello, Gopher! [200]
curl -s -w ' [%{http_code}]\n' localhost:8080/panic            # {"error":{"code":"internal",...}} [500]
curl -s -w ' [%{http_code}]\n' 'localhost:8080/slow?ms=1500'   # done [200]  (start it, then stop the server)
```

## Suggested steps

1. `Healthz`, `Hello` (`r.PathValue`), then `SlowDelay` and `Slow`. Run the handler tests first.
2. `RequestID`, then `Recover`, then `Logging`: after each one rerun the tests.
3. `NewServer`, then `Run`. Finish with the three shutdown tests.

<details>
<summary>Hints</summary>

- `Slow`: `time.NewTimer` plus `select` on `timer.C` and `r.Context().Done()`; never `time.Sleep` in a handler.
- `strconv.Atoi` accepts `"+5"` and rejects `"1.5"`; `min(a, b)` works on `time.Duration`.
- Request ID: `crypto/rand.Read` into 8 bytes, `hex.EncodeToString`. Put it in the context under an unexported key type.
- `Logging` must wrap `w` in `*statusRecorder` (an unwrapped `ResponseWriter` cannot tell you the status). Remember that a handler that never calls `WriteHeader` means `200`.
- `Recover` only sees the panic if it is the *innermost* of the two wrappers; that is why `Chain` lists it last. `recover()` only works when called directly inside the deferred function.
- `Run`: `srv.Serve(ln)` returns `http.ErrServerClosed` as soon as `Shutdown` starts, which is **not** an error. `Shutdown` needs a fresh `context.WithTimeout(context.Background(), drain)` because `ctx` is already cancelled.
- The log line is written after the response is flushed, so a client can read the response before the line exists; the tests poll for it.

</details>

## Stretch goals (not tested)

- Add a `GET /debug/vars` (`expvar`) listener on a **separate** loopback-only port.
- Log the request ID with `slog.Logger.With` in a handler via `RequestIDFrom`.
- Add `GET /readyz` that answers `503` as soon as shutdown starts (see `Server.RegisterOnShutdown`).

## Reference solution

[`_solution/`](./_solution/): copy it over the starter to see every test pass:

```bash
cp -r projects/P1-hello-http/_solution/. projects/P1-hello-http/starter/
```
