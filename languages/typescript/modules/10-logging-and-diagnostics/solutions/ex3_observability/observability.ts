import { randomUUID } from 'node:crypto';
import Fastify, { type FastifyBaseLogger, type FastifyInstance, LogController } from 'fastify';
import pino from 'pino';

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

declare module 'fastify' {
  interface FastifyRequest {
    startedAt: number;
  }
}

const REQUEST_ID_PATTERN = /^[\w.-]{1,64}$/;

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
export function buildApp({
  sink,
  now,
  generateId = randomUUID,
  checks = {},
}: ObservabilityOptions): FastifyInstance {
  const logger: FastifyBaseLogger = pino({ base: null, timestamp: false }, sink);
  const app = Fastify({
    loggerInstance: logger,
    logController: new LogController({
      disableRequestLogging: true,
      requestIdLogLabel: 'request_id',
    }),
    requestIdHeader: false,
    genReqId: (req) => {
      const header = req.headers['x-request-id'];
      return typeof header === 'string' && REQUEST_ID_PATTERN.test(header) ? header : generateId();
    },
  });

  const counts = new Map<string, number>();
  app.decorateRequest('startedAt', 0);

  app.addHook('onRequest', async (req, reply) => {
    req.startedAt = now();
    reply.header('x-request-id', req.id);
  });

  app.addHook('onResponse', async (req, reply) => {
    const route = req.routeOptions.url ?? 'unmatched';
    const key = `method="${req.method}",route="${route}",status="${reply.statusCode}"`;
    counts.set(key, (counts.get(key) ?? 0) + 1);
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

  app.setNotFoundHandler((_req, reply) =>
    reply.code(404).send({ error: { code: 'not_found', message: 'route not found' } }),
  );

  app.setErrorHandler((err: Error, req, reply) => {
    req.log.error({ err }, 'unhandled error');
    return reply.code(500).send({ error: { code: 'internal', message: 'internal server error' } });
  });

  app.get('/healthz', async () => ({ status: 'ok' }));

  app.get('/readyz', async (_req, reply) => {
    const results = await Promise.allSettled(Object.values(checks).map((check) => check()));
    const names = Object.keys(checks);
    const failed = names.filter((_, i) => results[i]?.status === 'rejected');
    if (failed.length > 0) return reply.code(503).send({ status: 'unavailable', failed });
    return { status: 'ready' };
  });

  app.get('/metrics', async (_req, reply) => {
    const lines = [...counts]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, n]) => `http_requests_total{${k}} ${n}`);
    return reply.type('text/plain; version=0.0.4').send(`${lines.join('\n')}\n`);
  });

  return app;
}
