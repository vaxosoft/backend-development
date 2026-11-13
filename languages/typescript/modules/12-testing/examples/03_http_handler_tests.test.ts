import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { buildApp, createMemoryRepo } from './_shared.ts';

const NOW = new Date('2030-01-01T09:00:00Z');

/** One app per test, closed when the test ends so the process never hangs. */
function makeApp(t: { after(fn: () => unknown): void }) {
  const repo = createMemoryRepo(() => NOW);
  const app = buildApp(repo);
  t.after(() => app.close());
  return { app, repo };
}

describe('POST /tasks', () => {
  test('creates a task: 201, Location and the full wire format', async (t) => {
    const { app } = makeApp(t);

    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      payload: { title: '  Write docs ', tags: ['Docs', 'p1', 'docs'], due_date: '2030-01-31' },
    });

    assert.equal(res.statusCode, 201);
    assert.equal(res.headers.location, '/tasks/1');
    assert.deepEqual(res.json(), {
      id: 1,
      title: 'Write docs',
      status: 'todo',
      due_date: '2030-01-31',
      tags: ['docs', 'p1'],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    });
  });

  test('invalid data: 422 with the shared envelope and field messages', async (t) => {
    const { app } = makeApp(t);

    const res = await app.inject({ method: 'POST', url: '/tasks', payload: { title: '   ' } });

    assert.equal(res.statusCode, 422);
    assert.deepEqual(res.json(), {
      error: {
        code: 'validation_failed',
        message: 'invalid task',
        fields: { title: 'must be 1-200 characters' },
      },
    });
  });

  test('malformed JSON: 400', async (t) => {
    const { app } = makeApp(t);

    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      headers: { 'content-type': 'application/json' },
      payload: '{"title":',
    });

    assert.equal(res.statusCode, 400);
    assert.equal(res.json().error.code, 'bad_request');
  });
});

describe('reading and deleting', () => {
  test('GET unknown id: 404 envelope', async (t) => {
    const { app } = makeApp(t);

    const res = await app.inject({ url: '/tasks/42' });

    assert.equal(res.statusCode, 404);
    assert.equal(res.json().error.code, 'not_found');
  });

  test('GET list filters by status and paginates', async (t) => {
    const { app, repo } = makeApp(t);
    for (const status of ['todo', 'done', 'todo', 'todo'] as const)
      repo.create({ title: status, status });

    const res = await app.inject({ url: '/tasks?status=todo&limit=2&offset=1' });
    const body = res.json();

    assert.equal(res.statusCode, 200);
    assert.equal(body.total, 3);
    assert.deepEqual(
      body.items.map((i: { id: number }) => i.id),
      [3, 1],
    );
    assert.equal(body.limit, 2);
    assert.equal(body.offset, 1);
  });

  test('DELETE: 204 once, then 404', async (t) => {
    const { app, repo } = makeApp(t);
    const { id } = repo.create({ title: 'x' });

    assert.equal((await app.inject({ method: 'DELETE', url: `/tasks/${id}` })).statusCode, 204);
    assert.equal((await app.inject({ method: 'DELETE', url: `/tasks/${id}` })).statusCode, 404);
  });
});
