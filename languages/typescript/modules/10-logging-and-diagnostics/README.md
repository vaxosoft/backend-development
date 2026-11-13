# 10. Logging and Diagnostics in TypeScript

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01, 02, 03 (Fastify routes, hooks and the error envelope)

## Learning Outcomes

After this module you can:

- [ ] Emit structured JSON logs with levels, child loggers and contextual fields using `pino`.
- [ ] Redact secrets so passwords and tokens never reach a log line.
- [ ] Attach a correlation (request) id to every line of a request without passing it around, using `AsyncLocalStorage`.
- [ ] Write request-logging and recovery middleware with Fastify hooks, and answer 500 without leaking details.
- [ ] Expose liveness, readiness and basic metrics endpoints, and know how to profile and debug a Node process.

## 1. Concept Overview

Logs are the first thing you read when production misbehaves, so they must be **structured** (one JSON object per line that machines can filter), **levelled** (debug < info < warn < error < fatal) and **correlated** (every line of one request shares an id). A request log line records the outcome (`method`, `path`, `status`, `duration_ms`); everything else is context added once and inherited.

Diagnostics go beyond logs: a **liveness** probe (`/healthz`, "the process is up"), a **readiness** probe (`/readyz`, "my dependencies work, send me traffic"), counters and timings (**metrics**), and the ability to attach a debugger or profiler to a running process. Recovery middleware turns an unexpected exception into a safe 500 response while the full error goes only to the log.

```text
request -> [onRequest: id, start time] -> handler -> [onResponse: log line + metrics]
                                             |
                                  throws --> error handler: log err, send generic 500
```

## 2. How TypeScript Approaches It

- **`pino`** is the structured logger Fastify uses natively. It writes JSON lines, is very fast and offers `child()`, `redact`, `serializers` and `mixin`. `pino-pretty` is for local reading only and is not used here.
- **Fastify hooks** (`onRequest`, `onResponse`) and `setErrorHandler` are the middleware. Fastify's built-in request lines are switched off with `new LogController({ disableRequestLogging: true })` so we log in our own, shared format; `requestIdLogLabel: 'request_id'` renames the field.
- **`node:async_hooks` `AsyncLocalStorage`** is Node's "context variable": a value that follows the logical flow of a request across `await`s. Plain module-level variables cannot do this, because many requests interleave on one thread.
- **`node:perf_hooks`** (`performance.now()`, `monitorEventLoopDelay`) for timings; `process.on('unhandledRejection' | 'uncaughtException')` for last-resort handlers; `node --inspect`, `--cpu-prof`, `--heap-prof` for debugging and profiling.
- Idiom: inject the log _destination_ (`pino(options, sink)`) and the _clock_, so tests can capture lines and fix durations. Avoid `console.log` in services: it is unstructured, unlevelled and cannot be redacted.

## 3. Guided Examples

All examples run from `languages/typescript/`. Shared helpers live in [examples/_shared.ts](examples/_shared.ts) (request-id resolution, the logging app factory, a tiny metrics registry).

### 3.1 pino basics: levels, child loggers, redaction

- **Goal:** structured fields, levels and secret redaction.
- **Run:** `node modules/10-logging-and-diagnostics/examples/01_pino_basics.ts`
- **Code:** [examples/01_pino_basics.ts](examples/01_pino_basics.ts)

```ts
const logger = pino({
  level: 'info',
  base: { service: 'task-tracker' },
  redact: { paths: ['password', '*.password', 'req.headers.authorization'], censor: '[REDACTED]' },
});
const requestLogger = logger.child({ request_id: 'req-42', user_id: 7 });
logger.info({ port: 8080 }, 'listening'); // object first, message second
```

- **Walkthrough:** `debug` is dropped because the level is `info`. The object argument becomes fields; the string becomes `msg`. `child()` returns a logger that adds its bindings to every line. `redact` replaces the value at the listed paths before serialising. The `err` key goes through a serializer (here trimmed so the output is stable; the default keeps the stack).
- **Expected output:**

```text
{"level":30,"service":"task-tracker","msg":"server starting"}
{"level":30,"service":"task-tracker","port":8080,"env":"dev","msg":"listening"}
{"level":40,"service":"task-tracker","retries":3,"msg":"slow dependency"}
{"level":30,"service":"task-tracker","request_id":"req-42","user_id":7,"status":200,"duration_ms":12,"msg":"request completed"}
{"level":30,"service":"task-tracker","user":{"email":"a@example.com","password":"[REDACTED]"},"msg":"user created"}
{"level":30,"service":"task-tracker","req":{"headers":{"authorization":"[REDACTED]"}},"msg":"incoming"}
{"level":50,"service":"task-tracker","err":{"type":"Error","message":"connection refused"},"msg":"could not reach database"}
```

### 3.2 A request id that follows the request: `AsyncLocalStorage`

- **Goal:** correlation ids without threading a logger through every function.
- **Run:** `node modules/10-logging-and-diagnostics/examples/02_request_context.ts`
- **Code:** [examples/02_request_context.ts](examples/02_request_context.ts)

```ts
const storage = new AsyncLocalStorage<RequestContext>();
const logger = pino({ mixin: () => ({ request_id: storage.getStore()?.requestId }) });
await storage.run({ requestId }, async () => {
  /* everything below sees requestId */
});
```

- **Walkthrough:** two simulated requests run concurrently. `loadTasks()` knows nothing about request ids, yet its lines carry the right one: `mixin` runs on each log call and reads the store of the current async flow. Outside `run`, `getStore()` is `undefined` and JSON drops the field.
- **Expected output:**

```text
{"level":30,"msg":"outside any request"}
{"level":30,"request_id":"req-a","msg":"request started"}
{"level":30,"request_id":"req-b","msg":"request started"}
{"level":30,"request_id":"req-a","msg":"loading tasks"}
{"level":30,"request_id":"req-a","msg":"tasks loaded"}
{"level":30,"request_id":"req-a","msg":"request finished"}
{"level":30,"request_id":"req-b","msg":"loading tasks"}
{"level":30,"request_id":"req-b","msg":"tasks loaded"}
{"level":30,"request_id":"req-b","msg":"request finished"}
```

### 3.3 Request logging, request ids and recovery with Fastify hooks

- **Goal:** the logging middleware: one line per request, a validated `X-Request-ID`, and a safe 500.
- **Run:** `node modules/10-logging-and-diagnostics/examples/03_request_logging.ts`
- **Code:** [examples/03_request_logging.ts](examples/03_request_logging.ts), built from `createApp` in [_shared.ts](examples/_shared.ts)

```ts
genReqId: (req) =>
  resolveRequestId(
    req.headers['x-request-id'],
  ), // validate, else randomUUID()
  app.addHook('onResponse', async (req, reply) => {
    req.log.info(
      {
        method: req.method,
        path: req.url.split('?')[0],
        status: reply.statusCode,
        duration_ms: Math.round(now() - req.startedAt),
      },
      'request completed',
    );
  });
app.setErrorHandler((err, req, reply) => {
  // recovery
  req.log.error({ err }, 'unhandled error');
  return reply.code(500).send(errorEnvelope('internal', 'internal server error'));
});
```

- **Walkthrough:** `req.log` is a child logger already bound to the request id. The path is logged without the query string (`?token=...` could hold secrets). The client header is reused only if it matches `[\w.-]{1,64}`, otherwise a UUID is generated, so a hostile header cannot forge or break log lines. Requests are injected (no network) and the clock advances 5 ms per call, so durations are stable. Generated UUIDs are random, so the demo masks them as `<generated-uuid>`.
- **Expected output:**

```text
200 req-123
500 {"error":{"code":"internal","message":"internal server error"}}
200 true
404 {"error":{"code":"not_found","message":"route not found"}}
--- log lines
{"level":30,"request_id":"req-123","method":"GET","path":"/tasks","status":200,"duration_ms":5,"msg":"request completed"}
{"level":50,"request_id":"<generated-uuid>","err":{"type":"Error","message":"connection to db:5432 refused"},"msg":"unhandled error"}
{"level":30,"request_id":"<generated-uuid>","method":"GET","path":"/boom","status":500,"duration_ms":5,"msg":"request completed"}
{"level":30,"request_id":"<generated-uuid>","method":"GET","path":"/tasks","status":200,"duration_ms":5,"msg":"request completed"}
{"level":30,"request_id":"req-404","method":"GET","path":"/nope","status":404,"duration_ms":5,"msg":"request completed"}
```

### 3.4 Health endpoints and metrics

- **Goal:** liveness vs readiness, and counters in the Prometheus text format.
- **Run:** `node modules/10-logging-and-diagnostics/examples/04_metrics_and_health.ts`
- **Code:** [examples/04_metrics_and_health.ts](examples/04_metrics_and_health.ts)
- **Walkthrough:** `/healthz` never touches dependencies (a failing database must not make the orchestrator restart a healthy process). `/readyz` runs named checks and answers 503 listing the failed ones. Metrics are labelled with the **route pattern** (`/tasks/:id`), not the URL, so `/tasks/1`, `/tasks/2`, ... stay one series. The duration sum/count lines let a dashboard compute the mean.
- **Expected output:**

```text
200 /healthz {"status":"ok"}
200 /readyz {"status":"ready"}
200 /tasks/1 {"id":1}
200 /tasks/2 {"id":2}
200 /tasks/3 {"id":3}
404 /nope {"error":{"code":"not_found","message":"route not found"}}
503 {"status":"unavailable","failed":["database"]}
--- /metrics
# TYPE http_requests_total counter
http_requests_total{method="GET",route="/healthz",status="200"} 1
http_requests_total{method="GET",route="/readyz",status="200"} 1
http_requests_total{method="GET",route="/readyz",status="503"} 1
http_requests_total{method="GET",route="/tasks/:id",status="200"} 3
http_requests_total{method="GET",route="unmatched",status="404"} 1
# TYPE http_request_duration_ms summary
http_request_duration_ms_sum 28
http_request_duration_ms_count 7
```

### 3.5 Task Tracker service with observability

- **Goal:** everything together in a real server (`ADDR` defaults to `127.0.0.1:8080`; Ctrl-C or SIGTERM closes it gracefully).
- **Run:** `ADDR=127.0.0.1:22350 node modules/10-logging-and-diagnostics/examples/05_task_tracker_observability.ts`
- **Code:** [examples/05_task_tracker_observability.ts](examples/05_task_tracker_observability.ts)
- **Walkthrough:** the server logs JSON (with an ISO `time`) to stdout. `process.on('uncaughtException' | 'unhandledRejection')` logs at `fatal` and exits non-zero: after such an event the process state is unknown, so let the supervisor restart it. Validation failures use the shared 422 envelope. In another terminal:

```bash
B=http://127.0.0.1:22350
curl -s -w '\n%{http_code} %header{x-request-id}\n' -X POST $B/tasks -H 'content-type: application/json' -H 'x-request-id: demo-1' -d '{"title":"Write docs"}'
curl -s -w '\n%{http_code}\n' -X POST $B/tasks -H 'content-type: application/json' -d '{"title":"   "}'
curl -s -w '\n%{http_code}\n' $B/tasks/99
curl -s -w '\n%{http_code}\n' $B/panic
curl -s $B/metrics | grep requests_total
```

- **Expected output:**

```text
{"id":1,"title":"Write docs","status":"todo"}
201 demo-1
{"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
422
{"error":{"code":"not_found","message":"task not found"}}
404
{"error":{"code":"internal","message":"internal server error"}}
500
# TYPE http_requests_total counter
http_requests_total{method="GET",route="/panic",status="500"} 1
http_requests_total{method="GET",route="/tasks/:id",status="404"} 1
http_requests_total{method="POST",route="/tasks",status="201"} 1
http_requests_total{method="POST",route="/tasks",status="422"} 1
```

The server's own log lines (stdout of the first terminal) have the shape shown in 3.3, plus `time`; the `/panic` line also carries the stack trace.

### Profiling and debugging basics

Not scripted, because the output is machine-specific:

| Need              | Command                                                                            |
| ----------------- | ---------------------------------------------------------------------------------- |
| Step through code | `node --inspect <file>.ts`, then open `chrome://inspect` or attach an IDE          |
| Restart on change | `node --watch <file>.ts`                                                           |
| CPU profile       | `node --cpu-prof <file>.ts` writes a `.cpuprofile` to open in DevTools             |
| Heap profile      | `node --heap-prof <file>.ts`                                                       |
| Event-loop lag    | `monitorEventLoopDelay()` from `node:perf_hooks`, export the histogram as a metric |

## 4. Common Pitfalls

| Mistake                                                             | Why it happens                                                                                                   | Fix                                                                                                       |
| ------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| Logging secrets (`password`, `Authorization`, whole request bodies) | Convenient to log "everything"                                                                                   | `redact` paths, log chosen fields only, never log `req.body` wholesale                                    |
| `console.log` in a service                                          | It works in dev                                                                                                  | Use the injected pino logger: levels, JSON, redaction, request id                                         |
| `logger.info('user', user)` or `logger.error(err)`                  | Argument order copied from `console`                                                                             | Object first, message second: `logger.error({ err }, 'msg')`; the `err` key triggers the error serializer |
| Trusting `X-Request-ID` blindly                                     | Header is client-controlled                                                                                      | Validate format and length, else generate (log forging, huge ids)                                         |
| Request id kept in a module-level variable                          | Looks simple                                                                                                     | Concurrent requests overwrite it: use `AsyncLocalStorage` (or `req.log`)                                  |
| Swallowing errors in a hook or error handler                        | `catch {}` to "keep going"                                                                                       | Log with `{ err }` then respond; an error handler that throws or logs nothing hides the bug               |
| Metrics labelled by raw URL or user id                              | Easy to implement                                                                                                | Use `req.routeOptions.url` (the pattern): unbounded labels blow up memory                                 |
| Readiness check inside `/healthz`                                   | One endpoint for both                                                                                            | Liveness must not depend on the database or the orchestrator will restart a healthy process               |
| Ignoring `unhandledRejection`                                       | An un-awaited promise rejects with nobody listening; since Node 15 that crashes the process without a useful log | Always `await` or `.catch` promises; install a last-resort handler that logs `fatal` and exits            |
| `logger.info(JSON.stringify(x))`                                    | Habit from `console.log`                                                                                         | Pass the object itself: pre-stringified JSON is double-escaped and unsearchable                           |

## 5. Coming From Other Languages

- **JavaScript:** same runtime; the new habit is leaving `console.log` behind.
- **Java/C#:** pino's `child()` and `mixin` play the role of SLF4J MDC / `ILogger.BeginScope`; `AsyncLocalStorage` is the thread-local replacement, because there are no per-request threads.
- **Python:** `logging` is string-oriented with formatters; pino is JSON-first. `contextvars` is the equivalent of `AsyncLocalStorage`.
- **Go:** `log/slog` with `With()` equals `child()`; Go passes a `context.Context` explicitly, Node propagates context implicitly.

## 6. Exercises

Each exercise has starter code and tests in [exercises/](exercises/); solutions are in [solutions/](solutions/).

| #   | Title                                                                | Difficulty | Skill practiced                             | Run tests                                                                                  |
| --- | -------------------------------------------------------------------- | ---------- | ------------------------------------------- | ------------------------------------------------------------------------------------------ |
| 1   | [Service logger](exercises/ex1_logger/logger.ts)                     | Easy       | pino options, redaction paths, levels       | `node --test "modules/10-logging-and-diagnostics/exercises/ex1_logger/*.test.ts"`          |
| 2   | [Request context](exercises/ex2_request_context/request-context.ts)  | Medium     | `AsyncLocalStorage`, `mixin`, id validation | `node --test "modules/10-logging-and-diagnostics/exercises/ex2_request_context/*.test.ts"` |
| 3   | [Observability plugin](exercises/ex3_observability/observability.ts) | Stretch    | hooks, error handler, health, metrics       | `node --test "modules/10-logging-and-diagnostics/exercises/ex3_observability/*.test.ts"`   |

**Exercise 1 - `createLogger`**

- **Task:** build a pino logger with a `service` base field (no pid/hostname), a configurable level and redaction of passwords, tokens and sensitive headers.
- **Acceptance:** all tests in `logger.test.ts` pass.
- <details><summary>Hint</summary>`redact.paths` accepts wildcards such as `*.password`, and `base` replaces the default bindings.</details>

**Exercise 2 - request context**

- **Task:** implement `resolveRequestId`, `runWithRequestContext`, `currentRequestId` and a logger that adds `request_id` automatically.
- **Acceptance:** concurrent requests keep their own ids; hostile ids are replaced; lines outside a request have no `request_id`.
- <details><summary>Hint</summary>One module-level `AsyncLocalStorage<string>`; the logger's `mixin` reads `getStore()`.</details>

**Exercise 3 - observability for a Fastify app**

- **Task:** implement `buildApp` so it provides request ids, one log line per request, recovery with a generic 500, `/healthz`, `/readyz` and `/metrics` as described in the file's doc comment.
- **Acceptance:** all tests in `observability.test.ts` pass (they use `inject()` and a fake clock; no network).
- <details><summary>Hint</summary>Start from example 3.3 and 3.4. Use `logController` to silence Fastify's own request lines and `req.routeOptions.url` for the metric label.</details>

## 7. Recap

- Log JSON, objects first, with levels; add context once through `child()` or `mixin`.
- Redact by path and log chosen fields: secrets must never be one `logger.info(req)` away.
- Correlate with a validated request id carried by `AsyncLocalStorage`.
- Recovery middleware logs the cause and answers with a generic 500; liveness and readiness are different questions.
- Use pino plus Fastify hooks for services; reach for `--inspect` and `--cpu-prof` when logs are not enough.

## 8. Self-check Questions

1. Why does a module-level `let currentRequestId` break under load but `AsyncLocalStorage` does not?
2. What is wrong with labelling a metric with `req.url`, and what is the safer label?
3. Why should `/healthz` not check the database, and what should `/readyz` do when it is down?
4. After an `uncaughtException`, why exit instead of continuing to serve?
5. Which pino call logs an error with its stack: `logger.error(err.message)`, `logger.error({ err }, 'x')` or `logger.error('x', err)`?

## 9. Further Reading

- pino documentation: [github.com/pinojs/pino](https://github.com/pinojs/pino) (`docs/redaction.md`, `docs/api.md`)
- Fastify reference: Logging and Hooks (`node_modules/fastify/docs/Reference/` in this repo)
- Node.js docs: `AsyncLocalStorage`, Diagnostics guide, `perf_hooks`

---

**Next:** [11 Authentication and Security Basics](../11-auth-and-security-basics/README.md)
