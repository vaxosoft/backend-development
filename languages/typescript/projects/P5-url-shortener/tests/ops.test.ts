import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import {
  bearer,
  createLink,
  login,
  register,
  signUp,
  startApp,
  type TestApp,
  waitForLogs,
} from './helpers.ts';

describe('rate limiting POST /links', () => {
  let t: TestApp;
  let alice: string;
  beforeEach(async () => {
    t = await startApp({ rateLimitBurst: 3, rateLimitRefillPerSecond: 0.5 });
    alice = await signUp(t.app, 'alice@example.com');
  });
  afterEach(() => t.close());

  const create = (token: string) => createLink(t.app, token, { url: 'https://example.com' });

  test('allows a burst, then answers 429 rate_limited with Retry-After', async () => {
    for (let i = 0; i < 3; i++) {
      assert.equal((await create(alice)).statusCode, 201, `request ${i + 1}`);
    }
    const limited = await create(alice);

    assert.equal(limited.statusCode, 429);
    assert.equal(limited.json().error.code, 'rate_limited');
    assert.equal(limited.headers['retry-after'], '2');
  });

  test('buckets are per user', async () => {
    const bob = await signUp(t.app, 'bob@example.com');
    for (let i = 0; i < 3; i++) {
      await create(alice);
    }

    assert.equal((await create(alice)).statusCode, 429);
    assert.equal((await create(bob)).statusCode, 201);
  });

  test('tokens refill over time', async () => {
    for (let i = 0; i < 3; i++) {
      await create(alice);
    }
    assert.equal((await create(alice)).statusCode, 429);

    t.advance(1000);
    assert.equal((await create(alice)).statusCode, 429, 'half a token is not enough');
    t.advance(1000);
    assert.equal((await create(alice)).statusCode, 201);
    assert.equal((await create(alice)).statusCode, 429);
  });

  test('public redirects and the listing are not rate limited', async () => {
    await createLink(t.app, alice, { url: 'https://example.com', slug: 'home' });
    for (let i = 0; i < 10; i++) {
      assert.equal((await t.app.inject({ method: 'GET', url: '/home' })).statusCode, 302);
      assert.equal(
        (await t.app.inject({ method: 'GET', url: '/links', headers: bearer(alice) })).statusCode,
        200,
      );
    }
  });
});

describe('structured request logging', () => {
  let t: TestApp;
  beforeEach(async () => {
    t = await startApp();
  });
  afterEach(() => t.close());

  test('writes one JSON line per request with the required fields', async () => {
    await t.app.inject({ method: 'GET', url: '/missing?secret=1' });
    await waitForLogs(t, 1);

    const lines = t.logs();
    assert.equal(lines.length, 1);
    const line = lines[0];
    assert.equal(line?.method, 'GET');
    assert.equal(line?.path, '/missing', 'the query string is not logged');
    assert.equal(line?.status, 404);
    assert.equal(typeof line?.request_id, 'string');
    assert.equal(typeof line?.duration_ms, 'number');
    assert.ok(!('user_id' in (line ?? {})), 'no user_id for anonymous requests');
  });

  test('includes user_id once the caller is authenticated', async () => {
    const token = await signUp(t.app, 'alice@example.com');
    await t.app.inject({ method: 'GET', url: '/links', headers: bearer(token) });
    await waitForLogs(t, 3);

    const last = t.logs().at(-1);
    assert.equal(last?.path, '/links');
    assert.equal(last?.status, 200);
    assert.equal(last?.user_id, 1);
  });

  test('echoes X-Request-ID and uses it as request_id, or generates one', async () => {
    const given = await t.app.inject({
      method: 'GET',
      url: '/a',
      headers: { 'x-request-id': 'req-abc' },
    });
    const generated = await t.app.inject({ method: 'GET', url: '/b' });
    await waitForLogs(t, 2);

    assert.equal(given.headers['x-request-id'], 'req-abc');
    const [first, second] = t.logs();
    assert.equal(first?.request_id, 'req-abc');
    assert.ok(generated.headers['x-request-id']);
    assert.equal(second?.request_id, generated.headers['x-request-id']);
  });

  test('never logs passwords or tokens', async () => {
    await register(t.app, 'alice@example.com', 'my very secret password');
    await login(t.app, 'alice@example.com', 'my very secret password');
    const token = (await login(t.app, 'alice@example.com', 'my very secret password')).json<{
      token: string;
    }>().token;
    await createLink(t.app, token, { url: 'https://example.com/?password=hunter2' });
    await t.app.inject({
      method: 'POST',
      url: '/auth/login',
      payload: { email: 'a@b.c', password: 'wrong secret' },
    });
    await waitForLogs(t, 5);

    const raw = t.rawLogs();
    assert.ok(t.logs().length >= 5);
    for (const secret of ['my very secret password', 'wrong secret', token, 'hunter2']) {
      assert.ok(!raw.includes(secret), `log contains ${secret.slice(0, 12)}`);
    }
  });
});
