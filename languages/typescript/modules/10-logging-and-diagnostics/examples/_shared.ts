// Helpers shared by examples 03-05. Not runnable.
import { randomUUID } from 'node:crypto';
import { performance } from 'node:perf_hooks';
import Fastify, { type FastifyBaseLogger, type FastifyInstance, LogController } from 'fastify';
import pino from 'pino';

export interface LogSink {
  write(line: string): void;
}

declare module 'fastify' {
  interface FastifyRequest {
    startedAt: number;
  }
}

export interface CompletedRequest {
  method: string;
  /** Route pattern such as `/tasks/:id`, or `unmatched`: never the raw URL (unbounded cardinality). */
  route: string;
  status: number;
  durationMs: number;
}

export interface AppOptions {
  sink: LogSink;
  onCompleted?: (request: CompletedRequest) => void;
  /** Monotonic clock in milliseconds; tests and demos inject a fake one. */
  now?: () => number;
  /** Drop timestamps and stack traces so demo output is stable (never do this in production). */
  deterministic?: boolean;
}

const REQUEST_ID_PATTERN = /^[\w.-]{1,64}$/;

/** Reuses a well-formed client id, otherwise generates one: never trust header content blindly. */
export function resolveRequestId(header: string | string[] | undefined): string {
  return typeof header === 'string' && REQUEST_ID_PATTERN.test(header) ? header : randomUUID();
}

export function errorEnvelope(code: string, message: string) {
  return { error: { code, message } };
}

const CODES: Record<number, string> = {
  400: 'bad_request',
  404: 'not_found',
  413: 'payload_too_large',
  422: 'validation_failed',
};

/** A Fastify app with a request id, one JSON log line per request and a safe 500 handler. */
export function createApp({
  sink,
  onCompleted,
  now = performance.now.bind(performance),
  deterministic = false,
}: AppOptions): FastifyInstance {
  const logger: FastifyBaseLogger = pino(
    {
      base: null,
      timestamp: deterministic ? false : pino.stdTimeFunctions.isoTime,
      ...(deterministic
        ? { serializers: { err: (e: Error) => ({ type: e.name, message: e.message }) } }
        : {}),
    },
    sink,
  );

  const app = Fastify({
    loggerInstance: logger,
    // We log the request ourselves, in the shared format, so the built-in lines are switched off.
    logController: new LogController({
      disableRequestLogging: true,
      requestIdLogLabel: 'request_id',
    }),
    requestIdHeader: false,
    genReqId: (req) => resolveRequestId(req.headers['x-request-id']),
  });

  app.decorateRequest('startedAt', 0);

  app.addHook('onRequest', async (req, reply) => {
    req.startedAt = now();
    reply.header('x-request-id', req.id);
  });

  app.addHook('onResponse', async (req, reply) => {
    const durationMs = Math.round(now() - req.startedAt);
    req.log.info(
      {
        method: req.method,
        path: req.url.split('?')[0],
        status: reply.statusCode,
        duration_ms: durationMs,
      },
      'request completed',
    );
    onCompleted?.({
      method: req.method,
      route: req.routeOptions.url ?? 'unmatched',
      status: reply.statusCode,
      durationMs,
    });
  });

  app.setNotFoundHandler((_req, reply) =>
    reply.code(404).send(errorEnvelope('not_found', 'route not found')),
  );

  // Recovery: whatever a handler throws, log the details server-side and answer with a generic body.
  app.setErrorHandler((err: Error & { statusCode?: number }, req, reply) => {
    const status = err.statusCode && err.statusCode < 500 ? err.statusCode : 500;
    if (status === 500) {
      req.log.error({ err }, 'unhandled error');
      return reply.code(500).send(errorEnvelope('internal', 'internal server error'));
    }
    return reply.code(status).send(errorEnvelope(CODES[status] ?? 'bad_request', err.message));
  });

  return app;
}

/** A deterministic clock for demos: every call advances by `stepMs`. */
export function steppingClock(stepMs: number): () => number {
  let t = 0;
  return () => {
    t += stepMs;
    return t;
  };
}

/** A sink that collects lines so a demo can print them after the fact. */
export function memorySink(): LogSink & { lines: string[] } {
  const lines: string[] = [];
  return {
    lines,
    write(line: string) {
      lines.push(line.trimEnd());
    },
  };
}

/** Counters in the Prometheus text exposition format, without any client library. */
export class Metrics {
  readonly #requests = new Map<string, number>();
  #durationSumMs = 0;
  #durationCount = 0;

  record({ method, route, status, durationMs }: CompletedRequest): void {
    const key = `method="${method}",route="${route}",status="${status}"`;
    this.#requests.set(key, (this.#requests.get(key) ?? 0) + 1);
    this.#durationSumMs += durationMs;
    this.#durationCount += 1;
  }

  render(): string {
    const lines = ['# TYPE http_requests_total counter'];
    for (const [labels, count] of [...this.#requests].sort(([a], [b]) => a.localeCompare(b))) {
      lines.push(`http_requests_total{${labels}} ${count}`);
    }
    lines.push(
      '# TYPE http_request_duration_ms summary',
      `http_request_duration_ms_sum ${this.#durationSumMs}`,
      `http_request_duration_ms_count ${this.#durationCount}`,
    );
    return `${lines.join('\n')}\n`;
  }
}

export type ReadinessCheck = () => Promise<void>;

/** `/healthz` says the process is alive; `/readyz` says its dependencies are usable. */
export function registerHealth(
  app: FastifyInstance,
  metrics: Metrics,
  checks: Record<string, ReadinessCheck> = {},
): void {
  app.get('/healthz', async () => ({ status: 'ok' }));

  app.get('/readyz', async (_req, reply) => {
    const failed: string[] = [];
    for (const [name, check] of Object.entries(checks)) {
      try {
        await check();
      } catch {
        failed.push(name);
      }
    }
    if (failed.length > 0) return reply.code(503).send({ status: 'unavailable', failed });
    return { status: 'ready' };
  });

  app.get('/metrics', async (_req, reply) =>
    reply.type('text/plain; version=0.0.4').send(metrics.render()),
  );
}
