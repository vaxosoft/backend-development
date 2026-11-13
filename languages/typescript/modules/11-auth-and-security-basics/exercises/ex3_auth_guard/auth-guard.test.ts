import assert from 'node:assert/strict';
import { describe, mock, test } from 'node:test';
import { buildApp, type Claims, type Take } from './auth-guard.ts';

const TOKENS: Record<string, Claims> = {
  'alice-token': { sub: 1, role: 'admin' },
  'bob-token': { sub: 2, role: 'member' },
};

function setup(take: (key: string) => Take = () => ({ allowed: true })) {
  const limiter = { take: mock.fn(take) };
  const verifyToken = mock.fn(async (token: string) => {
    if (token === 'explodes') throw new Error('verifier crashed');
    return TOKENS[token] ?? null;
  });
  const app = buildApp({ verifyToken, limiter });
  return { app, limiter, verifyToken };
}

const bearer = (token: string) => ({ authorization: `Bearer ${token}` });
const post = (headers: Record<string, string>, payload: unknown) => ({
  method: 'POST' as const,
  url: '/tasks',
  headers,
  payload: payload as object,
});

describe('authentication', () => {
  test('/healthz is public', async (t) => {
    const { app, verifyToken } = setup();
    t.after(() => app.close());

    const res = await app.inject({ url: '/healthz' });

    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), { status: 'ok' });
    assert.equal(verifyToken.mock.callCount(), 0);
  });

  test('valid bearer token identifies the user, scheme is case-insensitive', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    const res = await app.inject({ url: '/me', headers: { authorization: 'bearer bob-token' } });

    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), { id: 2, role: 'member' });
  });

  test('missing, wrong-scheme, empty, unknown and crashing credentials all give 401', async (t) => {
    const { app, verifyToken } = setup();
    t.after(() => app.close());

    const attempts: Record<string, string>[] = [
      {},
      { authorization: 'Basic YWxpY2U6cHc=' },
      { authorization: 'Bearer' },
      { authorization: 'Bearer ' },
      { authorization: 'Bearer nobody' },
      { authorization: 'Bearer explodes' },
    ];
    for (const headers of attempts) {
      const res = await app.inject({ url: '/me', headers });
      assert.equal(res.statusCode, 401, JSON.stringify(headers));
      assert.equal(res.headers['www-authenticate'], 'Bearer');
      assert.deepEqual(res.json(), {
        error: { code: 'unauthorized', message: 'missing or invalid token' },
      });
    }
    // The Basic credential must never be handed to the token verifier.
    assert.ok(!verifyToken.mock.calls.some((c) => c.arguments[0] === 'YWxpY2U6cHc='));
  });
});

describe('authorization', () => {
  test('admins reach /admin/stats, members get 403, anonymous gets 401', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    const admin = await app.inject({ url: '/admin/stats', headers: bearer('alice-token') });
    assert.equal(admin.statusCode, 200);
    assert.deepEqual(admin.json(), { users: 2 });

    const member = await app.inject({ url: '/admin/stats', headers: bearer('bob-token') });
    assert.equal(member.statusCode, 403);
    assert.equal((member.json() as { error: { code: string } }).error.code, 'forbidden');

    assert.equal((await app.inject({ url: '/admin/stats' })).statusCode, 401);
  });
});

describe('rate limiting on POST /tasks', () => {
  test('creates a task owned by the caller and charges that user', async (t) => {
    const { app, limiter } = setup();
    t.after(() => app.close());

    const res = await app.inject(post(bearer('bob-token'), { title: '  Write docs ' }));

    assert.equal(res.statusCode, 201);
    assert.deepEqual(res.json(), { id: 1, title: 'Write docs', owner_id: 2 });
    assert.deepEqual(
      limiter.take.mock.calls.map((c) => c.arguments[0]),
      ['user:2'],
    );
  });

  test('answers 429 with Retry-After when the limiter refuses', async (t) => {
    const { app } = setup(() => ({ allowed: false, retryAfterSeconds: 7 }));
    t.after(() => app.close());

    const res = await app.inject(post(bearer('bob-token'), { title: 'x' }));

    assert.equal(res.statusCode, 429);
    assert.equal(res.headers['retry-after'], '7');
    assert.equal((res.json() as { error: { code: string } }).error.code, 'rate_limited');
  });

  test('does not spend budget for anonymous requests, and limits before validating', async (t) => {
    const { app, limiter } = setup((key) =>
      key === 'user:2' ? { allowed: false, retryAfterSeconds: 1 } : { allowed: true },
    );
    t.after(() => app.close());

    assert.equal((await app.inject(post({}, { title: 'x' }))).statusCode, 401);
    assert.equal(limiter.take.mock.callCount(), 0);

    const invalidButLimited = await app.inject(post(bearer('bob-token'), { title: '' }));
    assert.equal(invalidButLimited.statusCode, 429);
  });

  test('validates the title', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    for (const payload of [{}, { title: '   ' }, { title: 'x'.repeat(201) }, { title: 5 }]) {
      const res = await app.inject(post(bearer('alice-token'), payload));
      assert.equal(res.statusCode, 422, JSON.stringify(payload));
    }
  });
});

describe('hardening headers', () => {
  test('nosniff everywhere, no-store on authenticated routes', async (t) => {
    const { app } = setup();
    t.after(() => app.close());

    const health = await app.inject({ url: '/healthz' });
    const me = await app.inject({ url: '/me', headers: bearer('bob-token') });
    const denied = await app.inject({ url: '/me' });
    const missing = await app.inject({ url: '/nope' });

    for (const res of [health, me, denied, missing]) {
      assert.equal(res.headers['x-content-type-options'], 'nosniff');
    }
    assert.equal(me.headers['cache-control'], 'no-store');
    assert.equal(denied.headers['cache-control'], 'no-store');
    assert.equal(health.headers['cache-control'], undefined);
  });
});
