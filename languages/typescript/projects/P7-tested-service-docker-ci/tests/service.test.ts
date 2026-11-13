import assert from 'node:assert/strict';
import { type TestContext, test } from 'node:test';
import type { FastifyInstance } from 'fastify';
import { type AppOptions, buildApp } from '../starter/src/app.ts';
import type { NewTask, Task, TaskStore } from '../starter/src/types.ts';

async function newApp(t: TestContext, options: AppOptions = {}): Promise<FastifyInstance> {
  const app = await buildApp(options);
  t.after(() => app.close());
  return app;
}

const frozenNow = () => new Date('2030-01-01T09:00:00.123Z');

test('GET /healthz answers 200 {"status":"ok"}', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({ url: '/healthz' });
  assert.equal(res.statusCode, 200);
  assert.deepEqual(res.json(), { status: 'ok' });
});

test('POST /tasks creates a task with defaults, Location and the injected clock', async (t) => {
  const app = await newApp(t, { now: frozenNow });
  const res = await app.inject({
    method: 'POST',
    url: '/tasks',
    payload: { title: '  Write docs ' },
  });
  assert.equal(res.statusCode, 201);
  assert.equal(res.headers.location, '/tasks/1');
  assert.deepEqual(res.json(), {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    due_date: null,
    tags: [],
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
  });
});

test('GET /tasks/{id} returns a task that was created before', async (t) => {
  const app = await newApp(t);
  await app.inject({ method: 'POST', url: '/tasks', payload: { title: 'first', tags: ['a'] } });
  const res = await app.inject({ url: '/tasks/1' });
  assert.equal(res.statusCode, 200);
  assert.equal(res.json<Task>().title, 'first');
  assert.deepEqual(res.json<Task>().tags, ['a']);
});

test('GET /tasks/{id} answers 404 with the error envelope when unknown', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({ url: '/tasks/7' });
  assert.equal(res.statusCode, 404);
  assert.equal(res.json<{ error: { code: string } }>().error.code, 'not_found');
});

test('POST /tasks rejects an invalid task with 422 and field errors', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({
    method: 'POST',
    url: '/tasks',
    payload: { title: '   ', tags: ['A', 'A'] },
  });
  assert.equal(res.statusCode, 422);
  const { error } = res.json<{ error: { code: string; fields: Record<string, string> } }>();
  assert.equal(error.code, 'validation_failed');
  assert.ok('title' in error.fields);
  assert.ok('tags' in error.fields);
});

test('the app talks to the injected TaskStore, and list paging is clamped to 100', async (t) => {
  const calls: Array<[number, number]> = [];
  const store: TaskStore = {
    create: (task: NewTask) => ({ id: 1, ...task }),
    get: () => undefined,
    list(limit, offset) {
      calls.push([limit, offset]);
      return { items: [], total: 0 };
    },
  };
  const app = await newApp(t, { store });
  const res = await app.inject({ url: '/tasks?limit=500&offset=3' });
  assert.equal(res.statusCode, 200);
  assert.deepEqual(res.json(), { items: [], total: 0, limit: 100, offset: 3 });
  assert.deepEqual(calls, [[100, 3]]);
});

test('GET /readyz is 503 until the app is marked ready', async (t) => {
  const app = await newApp(t);
  const before = await app.inject({ url: '/readyz' });
  assert.equal(before.statusCode, 503);
  assert.deepEqual(before.json(), { status: 'not_ready' });

  app.readiness.markReady();
  const after = await app.inject({ url: '/readyz' });
  assert.equal(after.statusCode, 200);
  assert.deepEqual(after.json(), { status: 'ready' });
});

test('GET /readyz goes back to 503 when shutdown starts', async (t) => {
  const app = await newApp(t);
  app.readiness.markReady();
  app.readiness.markNotReady();
  const res = await app.inject({ url: '/readyz' });
  assert.equal(res.statusCode, 503);
});

test('GET /metrics is plain text and counts every request including itself', async (t) => {
  const app = await newApp(t);
  await app.inject({ url: '/healthz' });
  await app.inject({ url: '/healthz' });
  await app.inject({ url: '/tasks/1' });
  const res = await app.inject({ url: '/metrics' });
  assert.equal(res.statusCode, 200);
  assert.match(String(res.headers['content-type']), /^text\/plain/);
  assert.equal(res.body, 'requests_total 4\n');
});

test('each app instance has its own request counter', async (t) => {
  const busy = await newApp(t);
  await busy.inject({ url: '/healthz' });
  await busy.inject({ url: '/healthz' });
  const fresh = await newApp(t);
  const res = await fresh.inject({ url: '/metrics' });
  assert.equal(res.body, 'requests_total 1\n');
});
