import assert from 'node:assert/strict';
import { test } from 'node:test';
import { buildApp } from './request-id.ts';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

test('a valid incoming X-Request-ID is echoed and visible to handlers', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({
    method: 'GET',
    url: '/whoami',
    headers: { 'x-request-id': 'client-id_1.2' },
  });
  assert.equal(res.headers['x-request-id'], 'client-id_1.2');
  assert.deepEqual(res.json(), { requestId: 'client-id_1.2' });
});

test('a missing id is generated and differs between requests', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const first = await app.inject({ method: 'GET', url: '/whoami' });
  const second = await app.inject({ method: 'GET', url: '/whoami' });
  const id = String(first.headers['x-request-id']);
  assert.match(id, UUID);
  assert.equal(first.json().requestId, id);
  assert.notEqual(second.headers['x-request-id'], id);
});

test('unsafe incoming ids are replaced', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  for (const bad of ['has space', 'x'.repeat(65), '<script>']) {
    const res = await app.inject({
      method: 'GET',
      url: '/whoami',
      headers: { 'x-request-id': bad },
    });
    assert.match(String(res.headers['x-request-id']), UUID, JSON.stringify(bad));
  }
});

test('the header is also set on 404 responses', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({
    method: 'GET',
    url: '/missing',
    headers: { 'x-request-id': 'abc' },
  });
  assert.equal(res.statusCode, 404);
  assert.equal(res.headers['x-request-id'], 'abc');
});

test('the admin hook protects /admin routes', async (t) => {
  const app = buildApp();
  t.after(() => app.close());

  const denied = await app.inject({ method: 'GET', url: '/admin/ping' });
  assert.equal(denied.statusCode, 401);
  assert.deepEqual(denied.json(), { error: 'unauthorized' });
  assert.match(String(denied.headers['x-request-id']), UUID);

  const allowed = await app.inject({
    method: 'GET',
    url: '/admin/ping',
    headers: { 'x-api-key': 'secret' },
  });
  assert.equal(allowed.statusCode, 200);
  assert.deepEqual(allowed.json(), { ok: true });
});

test('the admin hook does not leak to routes outside the plugin', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({ method: 'GET', url: '/public' });
  assert.equal(res.statusCode, 200);
  assert.deepEqual(res.json(), { visible: true });
});
