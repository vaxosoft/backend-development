# Shared Domain and Capstone Specifications

Every language track uses these definitions so lessons and projects line up across Go, Rust and C#. Idioms differ; the **wire format** and **behaviour** do not (except where noted).

## 1. Task Tracker domain

**User**: `id` (int), `name` (string, 1-100), `email` (string, contains `@`).

**Task**:

| Field | Type | Rules |
|-------|------|-------|
| `id` | int (server-assigned) | read-only |
| `title` | string | required, 1-200 chars (trimmed) |
| `status` | enum | `todo` (default), `in_progress`, `done` |
| `due_date` | `YYYY-MM-DD` or `null` | optional |
| `tags` | string[] | 0-10 tags, each 1-30 chars, lowercase, no duplicates |
| `owner_id` | int or `null` | optional |
| `created_at` | RFC 3339 UTC timestamp | read-only |

JSON uses `snake_case` field names in every language. Example:

```json
{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
```

## 2. REST conventions (Go, Rust, TypeScript and Python)

- Success: `200` (read/update), `201` + `Location` (create), `204` (delete).
- Errors use one JSON envelope with `Content-Type: application/json`:

```json
{"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
```

  | Situation | Status | `code` |
  |-----------|--------|--------|
  | malformed JSON / bad query value | 400 | `bad_request` |
  | missing/invalid credentials | 401 | `unauthorized` |
  | authenticated but not allowed | 403 | `forbidden` |
  | unknown resource | 404 | `not_found` |
  | well-formed but invalid data | 422 | `validation_failed` (with `fields`) |
  | rate limited | 429 | `rate_limited` (+ `Retry-After`) |
  | payload too large | 413 | `payload_too_large` |
  | unexpected | 500 | `internal` (never leak details) |

- Lists: `{"items":[...],"total":N,"limit":L,"offset":O}`; `limit` default 20, max 100 (larger values are clamped), `offset` default 0.
- **C#** teaches the idiomatic equivalent: `application/problem+json` (`ProblemDetails` / `ValidationProblemDetails`, RFC 9457) with the same status codes, and `{"items","total","limit","offset"}` for lists. Lessons say so explicitly.

## 3. Capstone projects

Each project lives in `projects/PN-name/{README.md, starter/, solution/, tests/}` (Rust: tests in `starter/tests/`). The README states requirements and acceptance criteria and lists the modules exercised. Tests are black-box over HTTP against an in-process server built by an entry-point function that the starter already defines (so tests compile against starter and solution alike). Tests must be deterministic, offline (loopback only), fast, and use SQLite in-memory or temp dirs. Starter code compiles and fails the tests.

Pick the language-natural name for the entry point (Go `New(cfg) http.Handler`, Rust `build_app(cfg) -> Router`, C# `public partial class Program` + `WebApplicationFactory`, TypeScript `buildApp(config): Promise<FastifyInstance>` in `starter/src/app.ts` exercised with `app.inject()` or a real listener on port 0, Python `create_app(config) -> FastAPI` in the `pN_name` package exercised with `TestClient` or a real uvicorn server on a free port), but keep behaviour identical. FastAPI's default 422 body and Fastify's default validation error body are replaced by the shared error envelope.

### P1-hello-http  (modules 01, 02, 10)
- `GET /healthz` -> 200 `{"status":"ok"}`
- `GET /hello/{name}` -> 200 `text/plain` `Hello, {name}!`; `GET /hello` -> `Hello, world!`
- `GET /slow?ms=N` -> sleeps N ms (max 2000) then 200 `done` (for shutdown tests)
- `GET /panic` -> handler panics/throws; the server must answer 500 with the error envelope and keep serving
- Middleware: request ID (`X-Request-ID` echoed or generated), request logging (one line per request to an injectable sink: method, path, status, duration), panic recovery
- Server timeouts configured; **graceful shutdown**: a function `run(addr/listener, shutdown_signal)` such that an in-flight `/slow` request finishes after shutdown starts, and new connections are refused afterwards.

### P2-tasks-crud-api  (modules 02-05)
- `POST /tasks` (201 + `Location: /tasks/{id}`), `GET /tasks/{id}`, `PUT /tasks/{id}` (full replace), `PATCH /tasks/{id}` (only `title`, `status`, `due_date`, `tags`; absent = unchanged; `due_date: null` clears), `DELETE /tasks/{id}` (204)
- `GET /tasks?status=&tag=&limit=&offset=&sort=` where `sort` is `created_at` or `-created_at` or `due_date`/`-due_date`; default `-created_at`
- Validation per section 1 -> 422; malformed JSON -> 400; unknown id -> 404
- SQLite persistence (schema created at startup/migration), repository abstraction, tags stored relationally or as JSON (free choice)
- `GET /openapi.json` is optional (C# only, built in); not required elsewhere.

### P3-file-service  (modules 02, 08)
- `POST /files` multipart field `file` -> streamed to disk (never fully buffered), 201 `{"id","name","size","sha256","content_type"}`; max size configurable (default 5 MiB) -> 413
- `GET /files/{id}` streams back with `Content-Length`, `Content-Type`, `Content-Disposition: attachment; filename="..."`, `ETag: "<sha256>"`; `If-None-Match` match -> 304
- `DELETE /files/{id}` -> 204; unknown id -> 404
- Stored under generated ids in a configurable directory; the client-supplied filename is sanitized (no path separators, `..`, control chars) and never used as a path. A name like `../../etc/passwd` must not escape the storage dir.

### P4-api-aggregator  (modules 06, 07)
- `GET /aggregate?ids=1,2,3` calls the configured upstream `GET {base}/items/{id}` for each id **concurrently**, with at most `max_concurrency` (default 4) in flight.
- Per-call timeout; retry up to 2 times on network error/5xx/timeout with exponential backoff (base configurable so tests use 1 ms); never retry 4xx.
- Response keeps request order: `{"results":[{"id":1,"data":<upstream JSON>}],"errors":[{"id":3,"error":"upstream status 500"}]}`; HTTP 200 even with partial failures; 400 if `ids` is missing/empty/over 50 entries/non-numeric.
- Client disconnect or overall deadline cancels outstanding upstream calls.
- Tests use a fake upstream server (counts concurrent calls, injects failures/delays).

### P5-url-shortener  (modules 03, 05, 10, 11)
- `POST /auth/register` `{"email","password"}` -> 201 (password min 8, stored hashed), `POST /auth/login` -> 200 `{"token":"<JWT>"}` (HS256, exp 1h; wrong credentials -> 401 with the same message for unknown user and wrong password)
- `POST /links` (auth) `{"url","slug"?}` -> 201 `{"slug","url"}`; url must be absolute http/https; slug optional (generated, 7 chars `[A-Za-z0-9]`), 3-32 chars `[A-Za-z0-9_-]`; duplicate slug -> 409 `conflict`
- `GET /{slug}` (public) -> 302 `Location`; unknown -> 404; hit counter incremented
- `GET /links` (auth, own links with `hits`), `DELETE /links/{slug}` (owner only: other user -> 403)
- Rate limit: token bucket per authenticated user on `POST /links` (config: burst, refill per second) -> 429 + `Retry-After`
- Structured logging: one JSON line per request with `request_id`, `method`, `path`, `status`, `duration_ms`, and `user_id` when known; secrets (password, token) never logged.

### P6-job-worker-pool  (modules 06, 10, 13)
- `POST /jobs` `{"type":"reverse"|"sleep"|"fail","payload":{...}}` -> 202 `{"id"}`; `reverse` payload `{"text"}` -> result is reversed text; `sleep` payload `{"ms"}` (cancellable, max 5000); `fail` always errors.
- `GET /jobs/{id}` -> `{"id","type","status":"queued|running|succeeded|failed|cancelled","attempts","result","error"}`; `GET /jobs` lists newest first
- Fixed pool of `workers` (default 4) pulling from a bounded queue (capacity configurable); queue full -> 429 `rate_limited`
- Failed jobs retry up to `max_attempts` (default 3) with a configurable (test: 1 ms) delay; each job has a timeout (config)
- `DELETE /jobs/{id}` cancels a queued or running job (204), finished job -> 409
- Graceful shutdown: stop accepting, let running/queued jobs finish (bounded by a drain timeout), then exit.

### P7-tested-service-docker-ci  (modules 09, 10, 12, 14)
- A small Tasks service (`GET/POST /tasks`, `GET /tasks/{id}`, in-memory store behind an interface) with `GET /healthz`, `GET /readyz` (503 until ready), `GET /metrics` (plain text `requests_total <n>`)
- Configuration from environment (`PORT`, `LOG_LEVEL` in debug|info|warn|error, `ENV`) with validation: invalid values make startup fail with a clear error (a function returning an error/Result, tested)
- `starter/Dockerfile`: multi-stage (>= 2 `FROM`), builds in a build stage, final stage minimal and runs as **non-root** (`USER`), `EXPOSE`, and `HEALTHCHECK` or documented probe; `starter/.dockerignore`
- `starter/.github/workflows/ci.yml`: triggers on push and pull_request; steps for restore/build, format check, lint, test; uses the pinned toolchain version
- Tests: behaviour tests (health/ready/metrics/config) plus "repo hygiene" tests that read `Dockerfile`, `.dockerignore` and `ci.yml` and assert the rules above. Docker itself is not run by the tests.
- Starter has the Tasks service but is missing readiness/metrics/config validation and the packaging files; the solution adds them.
