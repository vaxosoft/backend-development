# P1 - Hello HTTP

**Modules exercised:** [01 Language fundamentals](../../modules/01-language-fundamentals/README.md), [02 HTTP server](../../modules/02-http-server/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md)

Build a tiny HTTP server with routing, three pieces of middleware (request ID, request logging, panic recovery), configured server timeouts and a **graceful shutdown** that lets in-flight requests finish.

## Requirements

| Endpoint            | Behaviour                                                                                                                                                                        |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /healthz`      | `200` `{"status":"ok"}` (already in the starter, as a pattern)                                                                                                                   |
| `GET /hello`        | `200` `text/plain` `Hello, world!`                                                                                                                                               |
| `GET /hello/{name}` | `200` `text/plain` `Hello, {name}!` (percent-encoding decoded)                                                                                                                   |
| `GET /slow?ms=N`    | waits N ms (`ms` defaults to 0, clamped to 2000), then `200` `done`; a negative, non-numeric or non-integer `ms` gives `400` `bad_request`                                       |
| `GET /panic`        | the handler throws; the client gets `500` `{"error":{"code":"internal","message":"internal server error"}}`, the real error is reported to `errorSink`, the server keeps serving |
| anything else       | `404` `{"error":{"code":"not_found",...}}`                                                                                                                                       |

Middleware:

- **Request ID:** every response (including errors) has `X-Request-ID`: the incoming value is echoed, otherwise a UUID is generated.
- **Request logging:** one JSON line per finished request to `config.logSink`: `request_id`, `method`, `path` (without query), `status`, `duration_ms` (number).
- **Recovery:** thrown errors become the shared error envelope; 4xx errors keep their status, everything else is a 500 that never leaks the message.

Server and shutdown:

- Timeouts come from the config: `requestTimeoutMs`, `headersTimeoutMs`, `keepAliveTimeoutMs`.
- `run(app, { host, port, graceMs }, signal)` listens and returns `{ port, stopped }`. When `signal` aborts: stop accepting connections, let in-flight requests finish, and force-close what is still open after `graceMs`. `stopped` resolves when everything is down. `port: 0` picks a free port.
- `src/main.ts` reads `ADDR` (`host:port`, default `127.0.0.1:8080`) and aborts on `SIGINT`/`SIGTERM`.

## Acceptance criteria (test names in [tests/hello-http.test.ts](./tests/hello-http.test.ts))

| Area          | Tests                                                                                                                                                                                                                                                                                                     |
| ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Routes        | `GET /healthz returns ...`, `GET /hello greets the world ...`, `GET /hello/{name} greets by name`, `... decodes percent-encoded names`, `GET /slow?ms=N answers "done"`, `GET /slow actually waits ...`, `GET /slow rejects a bad ms ...`, `unknown routes return the 404 not_found envelope`             |
| Recovery      | `GET /panic answers 500 ... leaks nothing`, `the real error behind a 500 goes to the error sink`, `the server keeps serving after a panic`                                                                                                                                                                |
| Request ID    | `an incoming X-Request-ID is echoed`, `a unique id is generated ...`, `error responses carry the request id too`                                                                                                                                                                                          |
| Logging       | `writes one JSON line per request ...`, `the logged path has no query string ...`, `failed and unknown requests are logged with their status`                                                                                                                                                             |
| Configuration | `server timeouts are configured`, `parseAddr splits host:port ...`                                                                                                                                                                                                                                        |
| Shutdown      | `run() serves requests on the bound port`, `an in-flight /slow request finishes after shutdown starts`, `new connections are refused ...`, `stopped resolves only after the in-flight request is done`, `connections still open after the grace period are force-closed`, `an already aborted signal ...` |

## How to run

From `languages/typescript/`:

```bash
node projects/P1-hello-http/starter/src/main.ts                  # default ADDR=127.0.0.1:8080
ADDR=127.0.0.1:9000 node projects/P1-hello-http/starter/src/main.ts
```

Then, in another terminal:

```bash
curl -i http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/hello/Ada
curl -i -H 'X-Request-ID: demo-1' http://127.0.0.1:8080/panic
curl http://127.0.0.1:8080/slow?ms=1500    # press Ctrl-C in the server terminal meanwhile
```

With the reference solution the answers are `{"status":"ok"}`, `Hello, Ada!`, a `500` with `x-request-id: demo-1` and the `internal` envelope, and `done` (the server prints `shut down cleanly` only after that response was delivered).

## How to run the tests

```bash
node --test "projects/P1-hello-http/tests/**/*.test.ts"
```

The tests import `../starter/src/app.ts`. Out of the box they fail (`Error: TODO` or failed assertions); your job is to turn them green by editing the files under `starter/src/`.

## Suggested steps

1. Add the three routes in `starter/src/routes.ts` (`/healthz` shows the style). Return a string for `text/plain`.
2. Make `registerRequestId` set the response header, and teach `buildApp` to take the incoming header (`requestIdHeader`, `genReqId`).
3. Implement `registerRequestLogger` with an `onResponse` hook.
4. Implement `registerRecovery` (`setErrorHandler`, `setNotFoundHandler`).
5. Pass the timeouts to `Fastify({...})` in `buildApp`.
6. Implement `run()` in `starter/src/server.ts`.

<details>
<summary>Hints</summary>

- A Fastify handler can simply `throw`; an `async` handler that rejects goes to the same error handler. In Node, an error thrown in a bare `setTimeout` callback is **not** caught by Fastify and would crash the process.
- `reply.elapsedTime` (milliseconds) is available in `onResponse`.
- `req.id` is the request ID Fastify generated or read from `requestIdHeader`.
- `error.statusCode` is set on Fastify's own client errors (400 for a bad body, ...); anything without it is a 500.
- `app.listen({ host, port })` resolves once listening; `app.server.address()` tells you the real port.
- `app.close()` stops the listener immediately and waits for in-flight requests, but an idle keep-alive connection can delay it. Reply with `Connection: close` to responses sent during shutdown (an `onSend` hook) so the connection is not reused.
- `app.server.closeAllConnections()` is the hammer for the grace period; do not forget to clear that timer.
- `sleep` from `node:timers/promises` is the easiest awaitable delay.

</details>

## Stretch goals (not tested)

- Reject a client-supplied `X-Request-ID` that is longer than 64 characters or has unusual characters.
- Add `GET /slow` cancellation: stop sleeping when the client disconnects (`req.raw.on('close', ...)` plus an `AbortSignal`).
- Log through `pino` (Fastify's built-in logger) instead of a hand-written sink and compare the output.
- Add a second `SIGINT` handler that exits immediately when pressed twice.

## Reference solution

The model answer lives in [solution/](./solution/): copy its files over `starter/` (same relative paths) and the tests pass. Try the project yourself first.
