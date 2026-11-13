import assert from 'node:assert/strict';
import { test } from 'node:test';
import { buildApp } from './routes.ts';

test('GET /healthz returns JSON', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({ method: 'GET', url: '/healthz' });
  assert.equal(res.statusCode, 200);
  assert.match(String(res.headers['content-type']), /^application\/json/);
  assert.deepEqual(res.json(), { status: 'ok' });
});

test('GET /hello and /hello/:name return plain text', async (t) => {
  const app = buildApp();
  t.after(() => app.close());

  const world = await app.inject({ method: 'GET', url: '/hello' });
  assert.equal(world.statusCode, 200);
  assert.match(String(world.headers['content-type']), /^text\/plain/);
  assert.equal(world.body, 'Hello, world!');

  const ada = await app.inject({ method: 'GET', url: '/hello/Ada' });
  assert.equal(ada.body, 'Hello, Ada!');
});

test('path parameters are URL-decoded and ?shout=true upper-cases', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({ method: 'GET', url: '/hello/Ada%20Lovelace?shout=true' });
  assert.equal(res.body, 'HELLO, ADA LOVELACE!');
  const quiet = await app.inject({ method: 'GET', url: '/hello/Ada?shout=false' });
  assert.equal(quiet.body, 'Hello, Ada!');
});

test('GET /items/:id converts the id and rejects non-integers', async (t) => {
  const app = buildApp();
  t.after(() => app.close());

  const good = await app.inject({ method: 'GET', url: '/items/42' });
  assert.equal(good.statusCode, 200);
  assert.deepEqual(good.json(), { id: 42 });

  for (const bad of ['abc', '1.5', '%20', '']) {
    const res = await app.inject({ method: 'GET', url: `/items/${bad}` });
    assert.equal(res.statusCode, 400, `id ${JSON.stringify(bad)}`);
    assert.deepEqual(res.json(), { error: 'id must be an integer' });
  }
});

test('POST /echo answers 201 with the parsed body', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({ method: 'POST', url: '/echo', payload: { n: [1, 2], ok: true } });
  assert.equal(res.statusCode, 201);
  assert.deepEqual(res.json(), { received: { n: [1, 2], ok: true } });
});

test('unknown routes give a JSON 404', async (t) => {
  const app = buildApp();
  t.after(() => app.close());
  const res = await app.inject({ method: 'GET', url: '/nope' });
  assert.equal(res.statusCode, 404);
  assert.match(String(res.headers['content-type']), /^application\/json/);
  assert.deepEqual(res.json(), { error: 'not found' });
});
