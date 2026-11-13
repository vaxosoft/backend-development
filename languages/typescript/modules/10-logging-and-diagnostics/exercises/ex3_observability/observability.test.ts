import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { buildApp } from './observability.ts';

function setup(checks?: Record<string, () => Promise<void>>) {
  const lines: string[] = [];
  const clock = { t: 1000 };
  const app = buildApp({
    sink: {
      write(line: string) {
        lines.push(line);
      },
    },
    now: () => clock.t,
    generateId: () => 'generated-1',
    ...(checks ? { checks } : {}),
  });
  const records = (): Record<string, unknown>[] => lines.map((l) => JSON.parse(l));
  return { app, clock, records };
}

describe('request id', () => {
  test('echoes a well-formed id and logs it', async (t) => {
    const { app, records } = setup();
    t.after(() => app.close());
    app.get('/ping', async () => 'pong');

    const res = await app.inject({ url: '/ping', headers: { 'x-request-id': 'client-42' } });

    assert.equal(res.headers['x-request-id'], 'client-42');
    assert.equal(records()[0]?.request_id, 'client-42');
  });

  test('replaces missing or malformed ids', async (t) => {
    const { app } = setup();
    t.after(() => app.close());
    app.get('/ping', async () => 'pong');

    const missing = await app.inject({ url: '/ping' });
    const hostile = await app.inject({ url: '/ping', headers: { 'x-request-id': 'bad id' } });

    assert.equal(missing.headers['x-request-id'], 'generated-1');
    assert.equal(hostile.headers['x-request-id'], 'generated-1');
  });
});

describe('request log line', () => {
  test('has method, path without query, status and measured duration', async (t) => {
    const { app, clock, records } = setup();
    t.after(() => app.close());
    app.get('/slow', async () => {
      clock.t += 37.4;
      return 'done';
    });

    await app.inject({ url: '/slow?token=secret' });

    const lines = records();
    assert.equal(lines.length, 1);
    assert.deepEqual(
      { ...lines[0], level: undefined },
      {
        level: undefined,
        request_id: 'generated-1',
        method: 'GET',
        path: '/slow',
        status: 200,
        duration_ms: 37,
        msg: 'request completed',
      },
    );
    assert.ok(!JSON.stringify(lines).includes('secret'));
  });
});

describe('recovery', () => {
  test('answers 500 with a generic envelope, logs the cause, keeps serving', async (t) => {
    const { app, records } = setup();
    t.after(() => app.close());
    app.get('/boom', async () => {
      throw new Error('db password is hunter2');
    });
    app.get('/ok', async () => 'fine');

    const boom = await app.inject({ url: '/boom' });
    assert.equal(boom.statusCode, 500);
    assert.deepEqual(boom.json(), {
      error: { code: 'internal', message: 'internal server error' },
    });
    assert.ok(!boom.body.includes('hunter2'));

    const errorLine = records().find((r) => r.level === 50);
    assert.equal(errorLine?.request_id, 'generated-1');
    assert.match(JSON.stringify(errorLine?.err), /db password is hunter2/);
    assert.equal(records().filter((r) => r.msg === 'request completed')[0]?.status, 500);

    assert.equal((await app.inject({ url: '/ok' })).statusCode, 200);
  });

  test('unknown routes use the error envelope', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    const res = await app.inject({ url: '/missing' });

    assert.equal(res.statusCode, 404);
    assert.deepEqual(res.json(), { error: { code: 'not_found', message: 'route not found' } });
  });
});

describe('health endpoints', () => {
  test('/healthz is always ok', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    const res = await app.inject({ url: '/healthz' });

    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), { status: 'ok' });
  });

  test('/readyz reports failing checks by name', async (t) => {
    let dbUp = true;
    const { app } = setup({
      database: async () => {
        if (!dbUp) throw new Error('down');
      },
      cache: async () => {},
    });
    t.after(() => app.close());

    const ready = await app.inject({ url: '/readyz' });
    assert.equal(ready.statusCode, 200);
    assert.deepEqual(ready.json(), { status: 'ready' });

    dbUp = false;
    const notReady = await app.inject({ url: '/readyz' });
    assert.equal(notReady.statusCode, 503);
    assert.deepEqual(notReady.json(), { status: 'unavailable', failed: ['database'] });
  });
});

describe('metrics', () => {
  test('counts by method, route pattern and status', async (t) => {
    const { app } = setup();
    t.after(() => app.close());
    app.get('/tasks/:id', async (req) => ({ id: (req.params as { id: string }).id }));

    await app.inject({ url: '/tasks/1' });
    await app.inject({ url: '/tasks/2' });
    await app.inject({ url: '/nowhere' });
    const res = await app.inject({ url: '/metrics' });

    assert.match(res.headers['content-type'] as string, /^text\/plain/);
    const lines = res.body.split('\n');
    assert.ok(
      lines.includes('http_requests_total{method="GET",route="/tasks/:id",status="200"} 2'),
    );
    assert.ok(lines.includes('http_requests_total{method="GET",route="unmatched",status="404"} 1'));
    assert.ok(!res.body.includes('/tasks/1'));
  });
});
