import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import type { FastifyInstance } from 'fastify';
import { buildApp } from '../starter/src/app.ts';
import { type Config, defaultConfig } from '../starter/src/config.ts';
import { type Behavior, FakeUpstream } from './fake-upstream.ts';

interface AggregateBody {
  results: { id: number; data: { id: number; name: string } }[];
  errors: { id: number; error: string }[];
}

async function eventually(condition: () => boolean, what: string): Promise<void> {
  const start = Date.now();
  while (!condition()) {
    assert.ok(Date.now() - start < 5000, `timed out waiting for: ${what}`);
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}

describe('API aggregator', () => {
  let upstream: FakeUpstream;
  let apps: FastifyInstance[];

  beforeEach(async () => {
    upstream = await FakeUpstream.start();
    apps = [];
  });

  afterEach(async () => {
    await Promise.all(apps.map((app) => app.close()));
    await upstream.close();
  });

  async function start(overrides: Partial<Config> = {}): Promise<FastifyInstance> {
    const app = await buildApp({
      ...defaultConfig,
      upstreamBaseUrl: upstream.baseUrl,
      backoffBaseMs: 1,
      ...overrides,
    });
    apps.push(app);
    return app;
  }

  async function aggregate(app: FastifyInstance, ids: string) {
    const response = await app.inject({ method: 'GET', url: `/aggregate?ids=${ids}` });
    return { status: response.statusCode, body: response.json<AggregateBody>(), response };
  }

  const ids = (items: { id: number }[]) => items.map((item) => item.id);
  const script = (fn: (id: number, attempt: number) => Behavior) => {
    upstream.script = fn;
  };

  describe('input validation', () => {
    test('missing ids returns 400 with the error envelope', async () => {
      const app = await start();
      const response = await app.inject({ method: 'GET', url: '/aggregate' });

      assert.equal(response.statusCode, 400);
      assert.match(String(response.headers['content-type']), /^application\/json/);
      assert.equal(response.json().error.code, 'bad_request');
      assert.equal(typeof response.json().error.message, 'string');
    });

    test('empty, non-numeric, negative, fractional and blank entries return 400', async () => {
      const app = await start();
      for (const bad of ['', '1,a', '1,,2', '-1', '1.5', '1,2,', '1%202']) {
        const response = await app.inject({ method: 'GET', url: `/aggregate?ids=${bad}` });
        assert.equal(response.statusCode, 400, `ids=${JSON.stringify(bad)}`);
        assert.equal(response.json().error.code, 'bad_request');
      }
      assert.equal(upstream.totalCalls, 0);
    });

    test('more than 50 ids returns 400 and calls nothing', async () => {
      const app = await start();
      const list = Array.from({ length: 51 }, (_, i) => i + 1).join(',');
      const response = await app.inject({ method: 'GET', url: `/aggregate?ids=${list}` });

      assert.equal(response.statusCode, 400);
      assert.equal(upstream.totalCalls, 0);
    });

    test('exactly 50 ids are accepted', async () => {
      const app = await start();
      const list = Array.from({ length: 50 }, (_, i) => i + 1).join(',');
      const { status, body } = await aggregate(app, list);

      assert.equal(status, 200);
      assert.equal(body.results.length, 50);
    });

    test('spaces around ids are ignored', async () => {
      const app = await start();
      const { status, body } = await aggregate(app, '1,%202%20,3');

      assert.equal(status, 200);
      assert.deepEqual(ids(body.results), [1, 2, 3]);
    });
  });

  describe('aggregation', () => {
    test('wraps the upstream JSON in data', async () => {
      const app = await start();
      const { status, body } = await aggregate(app, '7');

      assert.equal(status, 200);
      assert.deepEqual(body, {
        results: [{ id: 7, data: { id: 7, name: 'item-7' } }],
        errors: [],
      });
    });

    test('keeps request order even when the upstream answers out of order', async () => {
      script((id) => ({ delayMs: id === 1 ? 120 : id === 2 ? 60 : 0 }));
      const app = await start();
      const { body } = await aggregate(app, '1,2,3');

      assert.deepEqual(ids(body.results), [1, 2, 3]);
    });

    test('partial failure is still 200 with results and errors in request order', async () => {
      script((id) => (id === 2 || id === 4 ? { status: 404 } : {}));
      const app = await start();
      const { status, body } = await aggregate(app, '1,2,3,4');

      assert.equal(status, 200);
      assert.deepEqual(ids(body.results), [1, 3]);
      assert.deepEqual(body.errors, [
        { id: 2, error: 'upstream status 404' },
        { id: 4, error: 'upstream status 404' },
      ]);
    });

    test('an invalid upstream JSON body is reported and not retried', async () => {
      script(() => ({ body: '{not json' }));
      const app = await start();
      const { body } = await aggregate(app, '1');

      assert.deepEqual(body.errors, [{ id: 1, error: 'invalid upstream response' }]);
      assert.equal(upstream.calls(1), 1);
    });
  });

  describe('concurrency', () => {
    test('never has more than maxConcurrency calls in flight', async () => {
      script(() => ({ delayMs: 40 }));
      const app = await start({ maxConcurrency: 2 });
      const { body } = await aggregate(app, '1,2,3,4,5,6');

      assert.equal(body.results.length, 6);
      assert.equal(upstream.maxInFlight, 2);
    });

    test('defaults to 4 concurrent calls and really runs them in parallel', async () => {
      script(() => ({ delayMs: 40 }));
      const app = await start();
      const { body } = await aggregate(app, '1,2,3,4,5,6,7,8');

      assert.equal(body.results.length, 8);
      assert.equal(upstream.maxInFlight, 4);
    });
  });

  describe('retries', () => {
    test('retries a 5xx and succeeds on the third attempt', async () => {
      script((_id, attempt) => (attempt < 3 ? { status: 503 } : {}));
      const app = await start();
      const { body } = await aggregate(app, '1');

      assert.deepEqual(ids(body.results), [1]);
      assert.equal(upstream.calls(1), 3);
    });

    test('gives up after two retries and reports the status', async () => {
      script(() => ({ status: 500 }));
      const app = await start();
      const { status, body } = await aggregate(app, '1');

      assert.equal(status, 200);
      assert.deepEqual(body.errors, [{ id: 1, error: 'upstream status 500' }]);
      assert.equal(upstream.calls(1), 3);
    });

    test('maxRetries is configurable', async () => {
      script(() => ({ status: 500 }));
      const none = await start({ maxRetries: 0 });
      await aggregate(none, '1');
      const many = await start({ maxRetries: 4 });
      await aggregate(many, '2');

      assert.equal(upstream.calls(1), 1);
      assert.equal(upstream.calls(2), 5);
    });

    test('backs off exponentially between attempts', async () => {
      script(() => ({ status: 500 }));
      const app = await start({ backoffBaseMs: 40 });
      const started = Date.now();
      await aggregate(app, '1');

      // 40 ms before the first retry plus 80 ms before the second
      assert.ok(Date.now() - started >= 110, 'retries must wait base * 2^n');
    });

    test('does not retry 4xx', async () => {
      script(() => ({ status: 404 }));
      const app = await start();
      const { body } = await aggregate(app, '1');

      assert.deepEqual(body.errors, [{ id: 1, error: 'upstream status 404' }]);
      assert.equal(upstream.calls(1), 1);
    });

    test('retries dropped connections and recovers', async () => {
      script((_id, attempt) => (attempt < 3 ? { drop: true } : {}));
      const app = await start();
      const { body } = await aggregate(app, '1');

      assert.deepEqual(ids(body.results), [1]);
      assert.equal(upstream.calls(1), 3);
    });

    test('reports upstream unreachable after repeated network errors', async () => {
      script(() => ({ drop: true }));
      const app = await start();
      const { status, body } = await aggregate(app, '1');

      assert.equal(status, 200);
      assert.deepEqual(body.errors, [{ id: 1, error: 'upstream unreachable' }]);
      assert.equal(upstream.calls(1), 3);
    });

    test('a per-call timeout is retried and finally reported as timeout', async () => {
      script(() => ({ stall: true }));
      const app = await start({ callTimeoutMs: 30 });
      const { body } = await aggregate(app, '1');

      assert.deepEqual(body.errors, [{ id: 1, error: 'timeout' }]);
      assert.equal(upstream.calls(1), 3);
    });
  });

  describe('cancellation', () => {
    test('the overall deadline cancels outstanding upstream calls', async () => {
      script((id) => (id === 2 ? { stall: true } : {}));
      const app = await start({ overallTimeoutMs: 100 });
      const { status, body } = await aggregate(app, '1,2');

      assert.equal(status, 200);
      assert.deepEqual(ids(body.results), [1]);
      assert.deepEqual(body.errors, [{ id: 2, error: 'deadline exceeded' }]);
      await eventually(() => upstream.cancelled.includes(2), 'upstream call 2 to be cancelled');
      assert.equal(upstream.calls(2), 1, 'no retry after the deadline');
    });

    test('ids still waiting for a slot are never started after the deadline', async () => {
      script(() => ({ stall: true }));
      const app = await start({ maxConcurrency: 1, overallTimeoutMs: 80 });
      const { body } = await aggregate(app, '1,2,3');

      assert.deepEqual(body.errors, [
        { id: 1, error: 'deadline exceeded' },
        { id: 2, error: 'deadline exceeded' },
        { id: 3, error: 'deadline exceeded' },
      ]);
      assert.equal(upstream.totalCalls, 1);
    });

    test('a client disconnect cancels outstanding upstream calls', async () => {
      script(() => ({ stall: true }));
      const app = await start({ maxConcurrency: 2 });
      const address = await app.listen({ host: '127.0.0.1', port: 0 });
      const client = new AbortController();

      const request = fetch(`${address}/aggregate?ids=1,2`, { signal: client.signal });
      request.catch(() => {});
      await eventually(() => upstream.inFlight === 2, 'both upstream calls to start');
      client.abort();

      await eventually(
        () => upstream.cancelled.length === 2,
        'both upstream calls to be cancelled',
      );
      assert.deepEqual([...upstream.cancelled].sort(), [1, 2]);
      assert.equal(upstream.totalCalls, 2, 'no retries after the client left');
    });
  });
});
