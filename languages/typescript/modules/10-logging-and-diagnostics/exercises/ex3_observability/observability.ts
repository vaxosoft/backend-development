import type { FastifyInstance } from 'fastify';

export interface LogSink {
  write(line: string): void;
}

export interface ObservabilityOptions {
  sink: LogSink;
  /** Monotonic clock in milliseconds. */
  now: () => number;
  /** Id factory for requests without a usable `X-Request-ID`. Defaults to `crypto.randomUUID`. */
  generateId?: () => string;
  /** Readiness checks by name; a check "fails" when its promise rejects. */
  checks?: Record<string, () => Promise<void>>;
}

/**
 * Builds a Fastify app (routes are added by the caller afterwards) that provides:
 *
 * 1. Request id: a client `X-Request-ID` made of 1-64 `[A-Za-z0-9_.-]` characters is reused,
 *    otherwise `generateId()` is used. The id is echoed in the `X-Request-ID` response header and
 *    attached to every log line of the request as `request_id`.
 * 2. One info line `request completed` per request with `method`, `path` (no query string),
 *    `status` and `duration_ms` (rounded, measured with `now`), written as JSON to `sink`.
 * 3. Recovery: an error thrown by a handler is logged at error level (field `err` with its message)
 *    and answered with 500 `{"error":{"code":"internal","message":"internal server error"}}`.
 *    Unknown routes answer 404 `{"error":{"code":"not_found","message":"route not found"}}`.
 * 4. `GET /healthz` -> 200 `{"status":"ok"}`.
 *    `GET /readyz` -> 200 `{"status":"ready"}`, or 503 `{"status":"unavailable","failed":[names]}`.
 * 5. `GET /metrics` -> `text/plain` with one line per (method, route pattern, status):
 *    `http_requests_total{method="GET",route="/tasks/:id",status="200"} 2`
 *    The route is the pattern (`/tasks/:id`), never the raw URL; unknown routes use `unmatched`.
 */
export function buildApp(_options: ObservabilityOptions): FastifyInstance {
  throw new Error('TODO');
}
