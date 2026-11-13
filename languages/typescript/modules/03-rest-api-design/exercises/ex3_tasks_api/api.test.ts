import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import type { FastifyInstance } from 'fastify';
import { buildApi } from './api.ts';
import { TaskStore } from './task-model.ts';

// Each task is created one second after the previous one, so sorting by created_at is predictable.
function newStore(): TaskStore {
  let seconds = 0;
  return new TaskStore(() => new Date(Date.UTC(2030, 0, 1, 9, 0, seconds++)));
}

async function withApi(
  t: { after: (fn: () => Promise<void>) => void },
  store: TaskStore = newStore(),
): Promise<FastifyInstance> {
  const app = buildApi(store);
  t.after(() => app.close());
  await app.ready();
  return app;
}

const json = { 'content-type': 'application/json' };

async function create(app: FastifyInstance, payload: object): Promise<{ id: number }> {
  const res = await app.inject({ method: 'POST', url: '/tasks', payload });
  assert.equal(res.statusCode, 201, res.body);
  return res.json();
}

function assertEnvelope(
  res: { statusCode: number; headers: Record<string, unknown>; json(): unknown },
  status: number,
  code: string,
) {
  assert.equal(res.statusCode, status);
  assert.match(String(res.headers['content-type']), /^application\/json/);
  const body = res.json() as {
    error: { code: string; message: string; fields?: Record<string, string> };
  };
  assert.equal(body.error.code, code);
  assert.equal(typeof body.error.message, 'string');
  return body.error;
}

describe('create and read', () => {
  test('POST /tasks answers 201 with Location and the stored task', async (t) => {
    const app = await withApi(t);
    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      payload: { title: '  Write docs ', tags: ['docs'] },
    });
    assert.equal(res.statusCode, 201);
    assert.equal(res.headers.location, '/tasks/1');
    assert.deepEqual(res.json(), {
      id: 1,
      title: 'Write docs',
      status: 'todo',
      due_date: null,
      tags: ['docs'],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    });
    const again = await app.inject({ method: 'GET', url: '/tasks/1' });
    assert.equal(again.statusCode, 200);
    assert.equal(again.json().title, 'Write docs');
  });

  test('unknown or malformed ids are 404 in the envelope', async (t) => {
    const app = await withApi(t);
    for (const id of ['99', 'abc', '1.5']) {
      assertEnvelope(await app.inject({ method: 'GET', url: `/tasks/${id}` }), 404, 'not_found');
    }
  });
});

describe('errors', () => {
  test('invalid body is 422 with a field map', async (t) => {
    const app = await withApi(t);
    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      payload: { title: '', tags: ['A'] },
    });
    const error = assertEnvelope(res, 422, 'validation_failed');
    assert.deepEqual(Object.keys(error.fields ?? {}).sort(), ['tags.0', 'title']);
  });

  test('malformed JSON is 400 bad_request', async (t) => {
    const app = await withApi(t);
    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      headers: json,
      payload: '{oops',
    });
    assertEnvelope(res, 400, 'bad_request');
  });

  test('a body above 1 KiB is 413 payload_too_large', async (t) => {
    const app = await withApi(t);
    const res = await app.inject({
      method: 'POST',
      url: '/tasks',
      payload: { title: 'x'.repeat(2000) },
    });
    assertEnvelope(res, 413, 'payload_too_large');
  });

  test('unknown routes use the envelope too', async (t) => {
    const app = await withApi(t);
    assertEnvelope(await app.inject({ method: 'GET', url: '/nope' }), 404, 'not_found');
  });

  test('unexpected errors become a generic 500 that leaks nothing', async (t) => {
    const store = newStore();
    store.list = () => {
      throw new Error('password=hunter2');
    };
    const app = await withApi(t, store);
    const res = await app.inject({ method: 'GET', url: '/tasks' });
    const error = assertEnvelope(res, 500, 'internal');
    assert.equal(error.message, 'internal server error');
    assert.ok(!res.body.includes('hunter2'));
  });
});

describe('update and delete', () => {
  test('PUT replaces the task, resetting omitted fields', async (t) => {
    const app = await withApi(t);
    await create(app, { title: 'Old', status: 'done', tags: ['x'], due_date: '2030-05-05' });
    const res = await app.inject({ method: 'PUT', url: '/tasks/1', payload: { title: 'New' } });
    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), {
      id: 1,
      title: 'New',
      status: 'todo',
      due_date: null,
      tags: [],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    });
    assertEnvelope(
      await app.inject({ method: 'PUT', url: '/tasks/1', payload: {} }),
      422,
      'validation_failed',
    );
    assertEnvelope(
      await app.inject({ method: 'PUT', url: '/tasks/9', payload: { title: 'x' } }),
      404,
      'not_found',
    );
  });

  test('PATCH changes only the given fields and null clears due_date', async (t) => {
    const app = await withApi(t);
    await create(app, { title: 'Keep', tags: ['a'], due_date: '2030-05-05', owner_id: 3 });

    const status = await app.inject({
      method: 'PATCH',
      url: '/tasks/1',
      payload: { status: 'done' },
    });
    assert.equal(status.json().status, 'done');
    assert.equal(status.json().title, 'Keep');
    assert.equal(status.json().due_date, '2030-05-05');

    const cleared = await app.inject({
      method: 'PATCH',
      url: '/tasks/1',
      payload: { due_date: null },
    });
    assert.equal(cleared.json().due_date, null);
    assert.deepEqual(cleared.json().tags, ['a']);
    assert.equal(cleared.json().owner_id, 3);

    assertEnvelope(
      await app.inject({ method: 'PATCH', url: '/tasks/1', payload: { status: 'x' } }),
      422,
      'validation_failed',
    );
    assertEnvelope(
      await app.inject({ method: 'PATCH', url: '/tasks/9', payload: { status: 'done' } }),
      404,
      'not_found',
    );
  });

  test('DELETE answers 204 once and 404 afterwards', async (t) => {
    const app = await withApi(t);
    await create(app, { title: 'Temporary' });
    const first = await app.inject({ method: 'DELETE', url: '/tasks/1' });
    assert.equal(first.statusCode, 204);
    assert.equal(first.body, '');
    assertEnvelope(await app.inject({ method: 'DELETE', url: '/tasks/1' }), 404, 'not_found');
    assertEnvelope(await app.inject({ method: 'GET', url: '/tasks/1' }), 404, 'not_found');
  });
});

describe('listing', () => {
  async function seeded(t: { after: (fn: () => Promise<void>) => void }): Promise<FastifyInstance> {
    const app = await withApi(t);
    await create(app, { title: 'A', status: 'todo', tags: ['docs'], due_date: '2030-03-01' });
    await create(app, { title: 'B', status: 'done', tags: ['docs', 'p1'] });
    await create(app, { title: 'C', status: 'todo', due_date: '2030-02-01' });
    return app;
  }

  const ids = (body: { items: { id: number }[] }): number[] => body.items.map((task) => task.id);

  test('default order is newest first, with the page envelope', async (t) => {
    const app = await seeded(t);
    const body = (await app.inject({ method: 'GET', url: '/tasks' })).json();
    assert.deepEqual(ids(body), [3, 2, 1]);
    assert.equal(body.total, 3);
    assert.equal(body.limit, 20);
    assert.equal(body.offset, 0);
  });

  test('filters by status and tag; total counts the matches', async (t) => {
    const app = await seeded(t);
    const todo = (
      await app.inject({ method: 'GET', url: '/tasks?status=todo&sort=created_at' })
    ).json();
    assert.deepEqual(ids(todo), [1, 3]);
    const docs = (await app.inject({ method: 'GET', url: '/tasks?tag=docs&limit=1' })).json();
    assert.deepEqual(ids(docs), [2]);
    assert.equal(docs.total, 2);
  });

  test('sorts by due_date with missing dates last', async (t) => {
    const app = await seeded(t);
    const asc = (await app.inject({ method: 'GET', url: '/tasks?sort=due_date' })).json();
    assert.deepEqual(ids(asc), [3, 1, 2]);
    const desc = (await app.inject({ method: 'GET', url: '/tasks?sort=-due_date' })).json();
    assert.deepEqual(ids(desc), [1, 3, 2]);
  });

  test('limit is clamped to 100 and offset pages through the list', async (t) => {
    const app = await seeded(t);
    const clamped = (await app.inject({ method: 'GET', url: '/tasks?limit=5000' })).json();
    assert.equal(clamped.limit, 100);
    const page = (
      await app.inject({ method: 'GET', url: '/tasks?limit=1&offset=1&sort=created_at' })
    ).json();
    assert.deepEqual(ids(page), [2]);
  });

  test('bad query values are 400', async (t) => {
    const app = await seeded(t);
    for (const query of ['limit=abc', 'limit=0', 'offset=-1', 'sort=title', 'status=nope']) {
      assertEnvelope(
        await app.inject({ method: 'GET', url: `/tasks?${query}` }),
        400,
        'bad_request',
      );
    }
  });
});

describe('idempotency', () => {
  test('repeating a POST with the same key replays the first response', async (t) => {
    const app = await withApi(t);
    const send = (title: string) =>
      app.inject({
        method: 'POST',
        url: '/tasks',
        headers: { 'idempotency-key': 'k1' },
        payload: { title },
      });

    const first = await send('Pay invoice');
    const second = await send('Pay invoice');
    assert.equal(first.statusCode, 201);
    assert.equal(second.statusCode, 201);
    assert.equal(first.headers['idempotent-replayed'], undefined);
    assert.equal(second.headers['idempotent-replayed'], 'true');
    assert.equal(second.headers.location, first.headers.location);
    assert.deepEqual(second.json(), first.json());

    const list = (await app.inject({ method: 'GET', url: '/tasks' })).json();
    assert.equal(list.total, 1);
  });

  test('the same key with a different body is a conflict', async (t) => {
    const app = await withApi(t);
    const send = (title: string, key: string) =>
      app.inject({
        method: 'POST',
        url: '/tasks',
        headers: { 'idempotency-key': key },
        payload: { title },
      });
    await send('One', 'k1');
    assertEnvelope(await send('Two', 'k1'), 409, 'conflict');
    assert.equal((await send('Two', 'k2')).statusCode, 201);
  });

  test('requests without a key always create new tasks', async (t) => {
    const app = await withApi(t);
    await create(app, { title: 'Same' });
    await create(app, { title: 'Same' });
    assert.equal((await app.inject({ method: 'GET', url: '/tasks' })).json().total, 2);
  });
});
