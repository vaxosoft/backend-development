import assert from 'node:assert/strict';
import { afterEach, describe, test } from 'node:test';
import { setImmediate as nextTick } from 'node:timers/promises';
import type { FastifyInstance } from 'fastify';
import { buildApp } from '../starter/src/app.ts';
import { parseAddr } from '../starter/src/config.ts';
import { run } from '../starter/src/server.ts';

interface Harness {
  app: FastifyInstance;
  logs: string[];
  errors: unknown[];
}

const open: FastifyInstance[] = [];

async function harness(): Promise<Harness> {
  const logs: string[] = [];
  const errors: unknown[] = [];
  const app = await buildApp({
    logSink: (line) => logs.push(line),
    errorSink: (error) => errors.push(error),
  });
  open.push(app);
  return { app, logs, errors };
}

// The access log is written after the response is flushed, so give it a moment.
async function logged(logs: string[], count: number): Promise<Record<string, unknown>[]> {
  for (let i = 0; i < 100 && logs.length < count; i++) await nextTick();
  return logs.map((line) => JSON.parse(line) as Record<string, unknown>);
}

afterEach(async () => {
  await Promise.all(open.splice(0).map((app) => app.close()));
});

describe('routes', () => {
  test('GET /healthz returns {"status":"ok"}', async () => {
    const { app } = await harness();
    const res = await app.inject('/healthz');
    assert.equal(res.statusCode, 200);
    assert.match(String(res.headers['content-type']), /^application\/json/);
    assert.deepEqual(res.json(), { status: 'ok' });
  });

  test('GET /hello greets the world as text/plain', async () => {
    const { app } = await harness();
    const res = await app.inject('/hello');
    assert.equal(res.statusCode, 200);
    assert.match(String(res.headers['content-type']), /^text\/plain/);
    assert.equal(res.body, 'Hello, world!');
  });

  test('GET /hello/{name} greets by name', async () => {
    const { app } = await harness();
    const res = await app.inject('/hello/Ada');
    assert.equal(res.statusCode, 200);
    assert.equal(res.body, 'Hello, Ada!');
  });

  test('GET /hello/{name} decodes percent-encoded names', async () => {
    const { app } = await harness();
    const res = await app.inject('/hello/Ada%20Lovelace');
    assert.equal(res.body, 'Hello, Ada Lovelace!');
  });

  test('GET /slow?ms=N answers "done"', async () => {
    const { app } = await harness();
    const res = await app.inject('/slow?ms=10');
    assert.equal(res.statusCode, 200);
    assert.equal(res.body, 'done');
  });

  test('GET /slow actually waits for N milliseconds', async () => {
    const { app } = await harness();
    const started = performance.now();
    await app.inject('/slow?ms=60');
    assert.ok(performance.now() - started >= 55);
  });

  test('GET /slow rejects a bad ms with a 400 bad_request envelope', async () => {
    const { app } = await harness();
    for (const ms of ['abc', '-5', '1.5']) {
      const res = await app.inject(`/slow?ms=${ms}`);
      assert.equal(res.statusCode, 400, `ms=${ms}`);
      assert.equal(res.json().error.code, 'bad_request');
    }
  });

  test('unknown routes return the 404 not_found envelope', async () => {
    const { app } = await harness();
    const res = await app.inject('/nope');
    assert.equal(res.statusCode, 404);
    assert.match(String(res.headers['content-type']), /^application\/json/);
    assert.equal(res.json().error.code, 'not_found');
  });
});

describe('panic recovery', () => {
  test('GET /panic answers 500 with the internal envelope and leaks nothing', async () => {
    const { app } = await harness();
    const res = await app.inject('/panic');
    assert.equal(res.statusCode, 500);
    assert.match(String(res.headers['content-type']), /^application\/json/);
    assert.equal(res.json().error.code, 'internal');
    assert.doesNotMatch(res.body, /boom/);
  });

  test('the real error behind a 500 goes to the error sink', async () => {
    const { app, errors } = await harness();
    await app.inject('/panic');
    assert.equal(errors.length, 1);
    assert.match(String((errors[0] as Error).message), /boom/);
  });

  test('the server keeps serving after a panic', async () => {
    const { app } = await harness();
    await app.inject('/panic');
    await app.inject('/panic');
    const res = await app.inject('/healthz');
    assert.equal(res.statusCode, 200);
  });
});

describe('request id', () => {
  test('an incoming X-Request-ID is echoed', async () => {
    const { app } = await harness();
    const res = await app.inject({ url: '/healthz', headers: { 'x-request-id': 'abc-123' } });
    assert.equal(res.headers['x-request-id'], 'abc-123');
  });

  test('a unique id is generated when the header is absent', async () => {
    const { app } = await harness();
    const a = await app.inject('/healthz');
    const b = await app.inject('/healthz');
    assert.ok(a.headers['x-request-id']);
    assert.notEqual(a.headers['x-request-id'], b.headers['x-request-id']);
  });

  test('error responses carry the request id too', async () => {
    const { app } = await harness();
    const res = await app.inject({ url: '/panic', headers: { 'x-request-id': 'req-500' } });
    assert.equal(res.headers['x-request-id'], 'req-500');
  });
});

describe('request logging', () => {
  test('writes one JSON line per request with method, path, status and duration', async () => {
    const { app, logs } = await harness();
    await app.inject({ url: '/hello/Ada', headers: { 'x-request-id': 'log-1' } });
    const [line, ...rest] = await logged(logs, 1);
    assert.equal(rest.length, 0);
    assert.equal(line?.request_id, 'log-1');
    assert.equal(line?.method, 'GET');
    assert.equal(line?.path, '/hello/Ada');
    assert.equal(line?.status, 200);
    assert.equal(typeof line?.duration_ms, 'number');
    assert.ok((line?.duration_ms as number) >= 0);
  });

  test('the logged path has no query string and the duration covers the handler', async () => {
    const { app, logs } = await harness();
    await app.inject('/slow?ms=60');
    const [line] = await logged(logs, 1);
    assert.equal(line?.path, '/slow');
    assert.ok((line?.duration_ms as number) >= 55);
  });

  test('failed and unknown requests are logged with their status', async () => {
    const { app, logs } = await harness();
    await app.inject('/panic');
    await app.inject('/nope');
    const lines = await logged(logs, 2);
    assert.deepEqual(
      lines.map((l) => l.status),
      [500, 404],
    );
  });
});

describe('server configuration', () => {
  test('server timeouts are configured', async () => {
    const app = await buildApp({
      requestTimeoutMs: 7_000,
      headersTimeoutMs: 3_000,
      keepAliveTimeoutMs: 2_000,
    });
    open.push(app);
    assert.equal(app.server.requestTimeout, 7_000);
    assert.equal(app.server.headersTimeout, 3_000);
    assert.equal(app.server.keepAliveTimeout, 2_000);
  });

  test('parseAddr splits host:port and rejects garbage', () => {
    assert.deepEqual(parseAddr('127.0.0.1:8080'), { host: '127.0.0.1', port: 8080 });
    assert.deepEqual(parseAddr('0.0.0.0:0'), { host: '0.0.0.0', port: 0 });
    assert.throws(() => parseAddr('8080'), /host:port/);
    assert.throws(() => parseAddr('localhost:http'), /host:port/);
    assert.throws(() => parseAddr('localhost:70000'), /host:port/);
  });
});

describe('graceful shutdown', () => {
  async function started(graceMs: number) {
    const { app } = await harness();
    const controller = new AbortController();
    const server = await run(app, { host: '127.0.0.1', port: 0, graceMs }, controller.signal);
    // Resolves as soon as the server has received a /slow request.
    const slowArrived = new Promise<void>((resolve) => {
      app.server.on('request', (req: { url?: string }) => {
        if (req.url?.startsWith('/slow')) resolve();
      });
    });
    return { app, controller, server, slowArrived, base: `http://127.0.0.1:${server.port}` };
  }

  test('run() serves requests on the bound port', async () => {
    const { controller, server, base } = await started(1_000);
    assert.ok(server.port > 0);
    const res = await fetch(`${base}/healthz`);
    assert.deepEqual(await res.json(), { status: 'ok' });
    controller.abort();
    await server.stopped;
  });

  test('an in-flight /slow request finishes after shutdown starts', async () => {
    const { controller, server, slowArrived, base } = await started(5_000);
    const inFlight = fetch(`${base}/slow?ms=150`);
    await slowArrived;
    controller.abort();
    const res = await inFlight;
    assert.equal(res.status, 200);
    assert.equal(await res.text(), 'done');
    await server.stopped;
  });

  test('new connections are refused once shutdown has completed', async () => {
    const { controller, server, base } = await started(1_000);
    controller.abort();
    await server.stopped;
    await assert.rejects(fetch(`${base}/healthz`));
  });

  test('stopped resolves only after the in-flight request is done', async () => {
    const { controller, server, slowArrived, base } = await started(5_000);
    const inFlight = fetch(`${base}/slow?ms=100`).then((res) => res.text());
    await slowArrived;
    const shutdownStarted = performance.now();
    controller.abort();
    await server.stopped;
    assert.ok(performance.now() - shutdownStarted >= 60);
    assert.equal(await inFlight, 'done');
  });

  test('connections still open after the grace period are force-closed', async () => {
    const { controller, server, slowArrived, base } = await started(30);
    const inFlight = fetch(`${base}/slow?ms=2000`);
    await slowArrived;
    controller.abort();
    await server.stopped;
    await assert.rejects(inFlight);
  });

  test('an already aborted signal shuts the server down immediately', async () => {
    const { app } = await harness();
    const controller = new AbortController();
    controller.abort();
    const server = await run(app, { host: '127.0.0.1', port: 0, graceMs: 100 }, controller.signal);
    await server.stopped;
    assert.equal(app.server.listening, false);
  });
});
