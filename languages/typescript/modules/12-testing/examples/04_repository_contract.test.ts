import assert from 'node:assert/strict';
import { describe, type TestContext, test } from 'node:test';
import { createMemoryRepo, type TaskRepo } from './_shared.ts';
import { createSqliteRepo } from './_sqlite-repo.ts';

const NOW = new Date('2030-01-01T09:00:00Z');

type Disposable = TaskRepo & { close?: () => void };

/**
 * The contract: behaviour every TaskRepo must have. The same tests run against each implementation,
 * so the fake used by fast unit tests cannot drift away from the real database.
 */
function repoContract(name: string, make: () => Disposable) {
  const fresh = (t: TestContext): TaskRepo => {
    const repo = make();
    t.after(() => repo.close?.());
    return repo;
  };

  describe(name, () => {
    test('create assigns increasing ids and applies defaults', (t) => {
      const repo = fresh(t);

      const first = repo.create({ title: 'a' });
      const second = repo.create({
        title: 'b',
        tags: ['x', 'y'],
        due_date: '2030-02-01',
        owner_id: 7,
      });

      assert.deepEqual(first, {
        id: 1,
        title: 'a',
        status: 'todo',
        due_date: null,
        tags: [],
        owner_id: null,
        created_at: '2030-01-01T09:00:00Z',
      });
      assert.equal(second.id, 2);
      assert.deepEqual(second.tags, ['x', 'y']);
      assert.equal(second.owner_id, 7);
    });

    test('get returns a copy, or undefined for an unknown id', (t) => {
      const repo = fresh(t);
      const { id } = repo.create({ title: 'a', tags: ['x'] });

      const loaded = repo.get(id);
      loaded?.tags.push('mutated');

      assert.deepEqual(repo.get(id)?.tags, ['x']);
      assert.equal(repo.get(999), undefined);
    });

    test('list is newest first, filters by status and reports the total', (t) => {
      const repo = fresh(t);
      for (const status of ['todo', 'done', 'todo', 'in_progress'] as const) {
        repo.create({ title: status, status });
      }

      assert.deepEqual(
        repo.list().items.map((x) => x.id),
        [4, 3, 2, 1],
      );
      const todos = repo.list({ status: 'todo' });
      assert.deepEqual(
        todos.items.map((x) => x.id),
        [3, 1],
      );
      assert.equal(todos.total, 2);
    });

    test('list paginates and clamps the limit', (t) => {
      const repo = fresh(t);
      for (let i = 0; i < 120; i++) repo.create({ title: `t${i}` });

      const page = repo.list({ limit: 2, offset: 1 });
      assert.deepEqual(
        page.items.map((x) => x.id),
        [119, 118],
      );
      assert.equal(page.total, 120);
      assert.equal(repo.list({ limit: 1000 }).items.length, 100);
      assert.equal(repo.list().items.length, 20);
    });

    test('update changes only the given fields and null clears the date', (t) => {
      const repo = fresh(t);
      const { id } = repo.create({ title: 'a', due_date: '2030-02-01', tags: ['x'] });

      const renamed = repo.update(id, { title: 'b' });
      assert.equal(renamed?.title, 'b');
      assert.equal(renamed?.due_date, '2030-02-01');
      assert.deepEqual(renamed?.tags, ['x']);

      assert.equal(repo.update(id, { due_date: null })?.due_date, null);
      assert.equal(repo.update(id, { status: 'done' })?.status, 'done');
      assert.equal(repo.update(999, { title: 'x' }), undefined);
    });

    test('delete reports whether something was removed', (t) => {
      const repo = fresh(t);
      const { id } = repo.create({ title: 'a' });

      assert.equal(repo.delete(id), true);
      assert.equal(repo.delete(id), false);
      assert.equal(repo.get(id), undefined);
    });
  });
}

repoContract('memory repo', () => createMemoryRepo(() => NOW));
repoContract('sqlite repo (:memory:)', () => createSqliteRepo(() => NOW));
