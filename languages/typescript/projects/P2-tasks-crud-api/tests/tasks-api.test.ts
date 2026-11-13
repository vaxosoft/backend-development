import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, describe, test } from 'node:test';
import type { FastifyInstance, LightMyRequestResponse } from 'fastify';
import { buildApp } from '../starter/src/app.ts';

const open: FastifyInstance[] = [];

// Each call to the clock is one second later, so creation order == created_at order.
function steppingClock(): () => Date {
  let seconds = 0;
  return () => new Date(Date.UTC(2030, 0, 1, 9, 0, seconds++));
}

async function newApp(dbPath = ':memory:'): Promise<FastifyInstance> {
  const app = await buildApp({ dbPath, clock: steppingClock() });
  open.push(app);
  return app;
}

afterEach(async () => {
  await Promise.all(open.splice(0).map((app) => app.close()));
});

async function post(app: FastifyInstance, body: unknown): Promise<LightMyRequestResponse> {
  return app.inject({ method: 'POST', url: '/tasks', payload: body as object });
}

async function seed(app: FastifyInstance, ...bodies: object[]): Promise<void> {
  for (const body of bodies) assert.equal((await post(app, body)).statusCode, 201);
}

const titles = (res: LightMyRequestResponse): string[] =>
  res.json().items.map((t: { title: string }) => t.title);

function assertEnvelope(res: LightMyRequestResponse, status: number, code: string): void {
  assert.equal(res.statusCode, status, res.body);
  assert.match(String(res.headers['content-type']), /^application\/json/);
  assert.equal(res.json().error.code, code);
  assert.equal(typeof res.json().error.message, 'string');
}

describe('POST /tasks', () => {
  test('creates a task: 201, Location header and defaults', async () => {
    const app = await newApp();
    const res = await post(app, { title: 'Write docs' });
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

  test('stores every writable field and trims the title', async () => {
    const app = await newApp();
    const res = await post(app, {
      title: '  Ship it  ',
      status: 'in_progress',
      due_date: '2030-01-31',
      tags: ['docs', 'p1'],
      owner_id: 7,
    });
    assert.equal(res.statusCode, 201);
    assert.deepEqual((await app.inject('/tasks/1')).json(), {
      id: 1,
      title: 'Ship it',
      status: 'in_progress',
      due_date: '2030-01-31',
      tags: ['docs', 'p1'],
      owner_id: 7,
      created_at: '2030-01-01T09:00:00Z',
    });
  });

  test('assigns increasing ids and ignores client-supplied id and created_at', async () => {
    const app = await newApp();
    await seed(app, { title: 'a' });
    const res = await post(app, { title: 'b', id: 99, created_at: '1999-01-01T00:00:00Z' });
    assert.equal(res.json().id, 2);
    assert.equal(res.json().created_at, '2030-01-01T09:00:01Z');
  });

  test('rejects a missing, blank or too long title with 422 and a fields map', async () => {
    const app = await newApp();
    for (const body of [{}, { title: '   ' }, { title: 'x'.repeat(201) }, { title: 42 }]) {
      const res = await post(app, body);
      assertEnvelope(res, 422, 'validation_failed');
      assert.equal(res.json().error.fields.title, 'must be 1-200 characters');
    }
    assert.equal((await post(app, { title: 'x'.repeat(200) })).statusCode, 201);
  });

  test('rejects an unknown status, a bad due_date and a non-integer owner_id', async () => {
    const app = await newApp();
    const cases: [string, object][] = [
      ['status', { status: 'finished' }],
      ['due_date', { due_date: '2030-02-31' }],
      ['due_date', { due_date: 'tomorrow' }],
      ['due_date', { due_date: '2030-1-5' }],
      ['due_date', { due_date: 20300101 }],
      ['owner_id', { owner_id: 'bob' }],
    ];
    for (const [field, extra] of cases) {
      const res = await post(app, { title: 't', ...extra });
      assertEnvelope(res, 422, 'validation_failed');
      assert.ok(res.json().error.fields[field], JSON.stringify(extra));
    }
  });

  test('rejects bad tags: uppercase, duplicates, empty, too long, more than 10', async () => {
    const app = await newApp();
    const bad = [
      ['Docs'],
      ['a', 'a'],
      [''],
      ['x'.repeat(31)],
      Array.from({ length: 11 }, (_, i) => `t${i}`),
      'docs',
    ];
    for (const tags of bad) {
      const res = await post(app, { title: 't', tags });
      assertEnvelope(res, 422, 'validation_failed');
      assert.ok(res.json().error.fields.tags, JSON.stringify(tags));
    }
    const ten = Array.from({ length: 10 }, (_, i) => `t${i}`);
    assert.equal((await post(app, { title: 't', tags: ten })).statusCode, 201);
  });

  test('malformed JSON is a 400 bad_request, a well-formed non-object is a 422', async () => {
    const app = await newApp();
    const malformed = await app.inject({
      method: 'POST',
      url: '/tasks',
      headers: { 'content-type': 'application/json' },
      payload: '{"title": ',
    });
    assertEnvelope(malformed, 400, 'bad_request');
    assertEnvelope(await post(app, [1, 2]), 422, 'validation_failed');
  });

  test('quotes in titles and tags are stored as data, not as SQL', async () => {
    const app = await newApp();
    await post(app, { title: "Robert'); DROP TABLE tasks;--", tags: ["it's"] });
    const res = await app.inject({ method: 'GET', url: '/tasks', query: { tag: "x' OR '1'='1" } });
    assert.equal(res.json().total, 0);
    assert.equal((await app.inject('/tasks/1')).json().title, "Robert'); DROP TABLE tasks;--");
  });
});

describe('GET /tasks/{id}', () => {
  test('returns the task; unknown and non-numeric ids are 404 not_found', async () => {
    const app = await newApp();
    await seed(app, { title: 'a', tags: ['x'] });
    const res = await app.inject('/tasks/1');
    assert.equal(res.statusCode, 200);
    assert.equal(res.json().title, 'a');
    assert.deepEqual(res.json().tags, ['x']);
    assertEnvelope(await app.inject('/tasks/42'), 404, 'not_found');
    assertEnvelope(await app.inject('/tasks/abc'), 404, 'not_found');
  });
});

describe('PUT /tasks/{id}', () => {
  test('replaces the whole task: omitted optional fields go back to their defaults', async () => {
    const app = await newApp();
    await seed(app, {
      title: 'old',
      status: 'done',
      due_date: '2030-05-05',
      tags: ['a'],
      owner_id: 3,
    });
    const res = await app.inject({ method: 'PUT', url: '/tasks/1', payload: { title: 'new' } });
    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), {
      id: 1,
      title: 'new',
      status: 'todo',
      due_date: null,
      tags: [],
      owner_id: null,
      created_at: '2030-01-01T09:00:00Z',
    });
    assert.deepEqual((await app.inject('/tasks/1')).json(), res.json());
  });

  test('validates the body (422) and reports unknown ids (404)', async () => {
    const app = await newApp();
    await seed(app, { title: 'a' });
    const invalid = await app.inject({ method: 'PUT', url: '/tasks/1', payload: { title: '' } });
    assertEnvelope(invalid, 422, 'validation_failed');
    const missing = await app.inject({ method: 'PUT', url: '/tasks/9', payload: { title: 'x' } });
    assertEnvelope(missing, 404, 'not_found');
  });
});

describe('PATCH /tasks/{id}', () => {
  test('changes only the supplied fields', async () => {
    const app = await newApp();
    await seed(app, { title: 'a', due_date: '2030-05-05', tags: ['x'], owner_id: 3 });
    const res = await app.inject({ method: 'PATCH', url: '/tasks/1', payload: { status: 'done' } });
    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.json(), {
      id: 1,
      title: 'a',
      status: 'done',
      due_date: '2030-05-05',
      tags: ['x'],
      owner_id: 3,
      created_at: '2030-01-01T09:00:00Z',
    });
  });

  test('due_date: null clears the date, while an absent due_date keeps it', async () => {
    const app = await newApp();
    await seed(app, { title: 'a', due_date: '2030-05-05' });
    const keep = await app.inject({ method: 'PATCH', url: '/tasks/1', payload: { title: 'b' } });
    assert.equal(keep.json().due_date, '2030-05-05');
    const clear = await app.inject({
      method: 'PATCH',
      url: '/tasks/1',
      payload: { due_date: null },
    });
    assert.equal(clear.json().due_date, null);
  });

  test('replaces tags and ignores fields that are not patchable', async () => {
    const app = await newApp();
    await seed(app, { title: 'a', tags: ['x'], owner_id: 3 });
    const res = await app.inject({
      method: 'PATCH',
      url: '/tasks/1',
      payload: { tags: ['y', 'z'], owner_id: 9, id: 5, created_at: '2000-01-01T00:00:00Z' },
    });
    assert.deepEqual(res.json().tags, ['y', 'z']);
    assert.equal(res.json().owner_id, 3);
    assert.equal(res.json().id, 1);
    assert.equal(res.json().created_at, '2030-01-01T09:00:00Z');
  });

  test('validates supplied fields (422) and reports unknown ids (404)', async () => {
    const app = await newApp();
    await seed(app, { title: 'a' });
    for (const payload of [{ title: null }, { title: ' ' }, { status: 'nope' }, { tags: ['A'] }]) {
      const res = await app.inject({ method: 'PATCH', url: '/tasks/1', payload });
      assertEnvelope(res, 422, 'validation_failed');
    }
    const missing = await app.inject({ method: 'PATCH', url: '/tasks/9', payload: { title: 'x' } });
    assertEnvelope(missing, 404, 'not_found');
  });
});

describe('DELETE /tasks/{id}', () => {
  test('answers 204 with no body, removes the task, and a second delete is 404', async () => {
    const app = await newApp();
    await seed(app, { title: 'a' });
    const res = await app.inject({ method: 'DELETE', url: '/tasks/1' });
    assert.equal(res.statusCode, 204);
    assert.equal(res.body, '');
    assertEnvelope(await app.inject('/tasks/1'), 404, 'not_found');
    assertEnvelope(await app.inject({ method: 'DELETE', url: '/tasks/1' }), 404, 'not_found');
  });
});

describe('GET /tasks', () => {
  test('returns the list envelope, newest first by default (empty store included)', async () => {
    const app = await newApp();
    assert.deepEqual((await app.inject('/tasks')).json(), {
      items: [],
      total: 0,
      limit: 20,
      offset: 0,
    });
    await seed(app, { title: 'first' }, { title: 'second' }, { title: 'third' });
    const res = await app.inject('/tasks');
    assert.equal(res.statusCode, 200);
    assert.equal(res.json().total, 3);
    assert.equal(res.json().limit, 20);
    assert.equal(res.json().offset, 0);
    assert.deepEqual(titles(res), ['third', 'second', 'first']);
  });

  test('filters by status and by tag, and combines the filters', async () => {
    const app = await newApp();
    await seed(
      app,
      { title: 'a', status: 'done', tags: ['x'] },
      { title: 'b', status: 'todo', tags: ['x', 'y'] },
      { title: 'c', status: 'done', tags: ['y'] },
    );
    const get = (query: Record<string, string>) =>
      app.inject({ method: 'GET', url: '/tasks', query });
    assert.deepEqual(titles(await get({ status: 'done' })), ['c', 'a']);
    assert.deepEqual(titles(await get({ tag: 'y' })), ['c', 'b']);
    assert.deepEqual(titles(await get({ tag: 'x', status: 'done' })), ['a']);
    assert.equal((await get({ tag: 'xx' })).json().total, 0);
  });

  test('paginates with limit and offset; total counts all matches', async () => {
    const app = await newApp();
    await seed(app, ...['a', 'b', 'c', 'd', 'e'].map((title) => ({ title })));
    const page = await app.inject({
      method: 'GET',
      url: '/tasks',
      query: { limit: '2', offset: '1' },
    });
    assert.deepEqual(titles(page), ['d', 'c']);
    assert.equal(page.json().total, 5);
    assert.equal(page.json().limit, 2);
    assert.equal(page.json().offset, 1);
    const past = await app.inject({ method: 'GET', url: '/tasks', query: { offset: '10' } });
    assert.deepEqual(past.json().items, []);
    assert.equal(past.json().total, 5);
  });

  test('a limit above 100 is clamped to 100', async () => {
    const app = await newApp();
    const res = await app.inject({ method: 'GET', url: '/tasks', query: { limit: '5000' } });
    assert.equal(res.statusCode, 200);
    assert.equal(res.json().limit, 100);
  });

  test('bad query values are 400 bad_request', async () => {
    const app = await newApp();
    const queries: Record<string, string>[] = [
      { limit: '0' },
      { limit: '-1' },
      { limit: 'ten' },
      { offset: '-5' },
      { offset: '1.5' },
      { status: 'finished' },
      { sort: 'title' },
    ];
    for (const query of queries) {
      const res = await app.inject({ method: 'GET', url: '/tasks', query });
      assertEnvelope(res, 400, 'bad_request');
    }
  });

  test('sorts by created_at ascending, and by due_date both ways with undated tasks last', async () => {
    const app = await newApp();
    await seed(
      app,
      { title: 'none' },
      { title: 'mid', due_date: '2030-02-01' },
      { title: 'late', due_date: '2030-03-01' },
      { title: 'early', due_date: '2030-01-15' },
    );
    const created = await app.inject({
      method: 'GET',
      url: '/tasks',
      query: { sort: 'created_at' },
    });
    assert.deepEqual(titles(created), ['none', 'mid', 'late', 'early']);
    const asc = await app.inject({ method: 'GET', url: '/tasks', query: { sort: 'due_date' } });
    assert.deepEqual(titles(asc), ['early', 'mid', 'late', 'none']);
    const desc = await app.inject({ method: 'GET', url: '/tasks', query: { sort: '-due_date' } });
    assert.deepEqual(titles(desc), ['late', 'mid', 'early', 'none']);
  });
});

describe('persistence', () => {
  test('tasks survive closing and reopening the database file', async () => {
    const dir = await mkdtemp(join(tmpdir(), 'p2-tasks-'));
    try {
      const dbPath = join(dir, 'tasks.db');
      const first = await buildApp({ dbPath, clock: steppingClock() });
      await seed(first, { title: 'durable', tags: ['keep'] });
      await first.close();

      const second = await newApp(dbPath);
      const res = await second.inject('/tasks/1');
      assert.equal(res.statusCode, 200);
      assert.equal(res.json().title, 'durable');
      assert.deepEqual(res.json().tags, ['keep']);
    } finally {
      await Promise.all(open.splice(0).map((app) => app.close()));
      await rm(dir, { recursive: true, force: true });
    }
  });
});
