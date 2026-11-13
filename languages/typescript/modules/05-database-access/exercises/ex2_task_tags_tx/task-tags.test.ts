import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import Database from 'better-sqlite3';
import { createTaskWithTags, renameTag } from './task-tags.ts';

let db: Database.Database;

beforeEach(() => {
  db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  db.exec(`
    CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL);
    CREATE TABLE task_tags (
      task_id INTEGER NOT NULL REFERENCES tasks(id),
      tag TEXT NOT NULL CHECK (length(tag) <= 30),
      PRIMARY KEY (task_id, tag)
    );
  `);
});

afterEach(() => {
  db.close();
});

const count = (table: string) => db.prepare(`SELECT count(*) FROM ${table}`).pluck().get();
const tagsOf = (id: number) =>
  db.prepare('SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag').pluck().all(id);

describe('createTaskWithTags', () => {
  test('stores the task and its tags and returns them sorted', () => {
    const task = createTaskWithTags(db, 'Write docs', ['p1', 'docs']);
    assert.deepEqual(task, { id: 1, title: 'Write docs', tags: ['docs', 'p1'] });
    assert.deepEqual(tagsOf(1), ['docs', 'p1']);
  });

  test('works without tags', () => {
    assert.deepEqual(createTaskWithTags(db, 'Plain', []), { id: 1, title: 'Plain', tags: [] });
  });

  test('a duplicate tag rolls back the task and the tags already written', () => {
    assert.throws(
      () => createTaskWithTags(db, 'Broken', ['a', 'b', 'a']),
      (err: unknown) =>
        err instanceof Database.SqliteError && err.code === 'SQLITE_CONSTRAINT_PRIMARYKEY',
    );
    assert.equal(count('tasks'), 0);
    assert.equal(count('task_tags'), 0);
    assert.equal(db.inTransaction, false);
  });

  test('a CHECK violation rolls back as well, and later calls still work', () => {
    assert.throws(
      () => createTaskWithTags(db, 'Long', ['ok', 'x'.repeat(31)]),
      Database.SqliteError,
    );
    assert.equal(count('tasks'), 0);
    assert.equal(createTaskWithTags(db, 'Fine', ['ok']).title, 'Fine');
    assert.equal(count('tasks'), 1);
  });

  test('inside an outer transaction a failure only undoes the inner work', () => {
    const outer = db.transaction(() => {
      createTaskWithTags(db, 'kept', ['a']);
      try {
        createTaskWithTags(db, 'dropped', ['b', 'b']);
      } catch {
        // the outer transaction decides to continue
      }
      createTaskWithTags(db, 'also kept', []);
    });
    outer();
    assert.deepEqual(db.prepare('SELECT title FROM tasks ORDER BY id').pluck().all(), [
      'kept',
      'also kept',
    ]);
    assert.equal(count('task_tags'), 1);
  });
});

describe('renameTag', () => {
  beforeEach(() => {
    createTaskWithTags(db, 'one', ['old']);
    createTaskWithTags(db, 'two', ['old', 'new']);
    createTaskWithTags(db, 'three', ['other']);
  });

  test('renames the tag everywhere and reports how many tasks changed', () => {
    assert.equal(renameTag(db, 'old', 'fresh'), 2);
    assert.deepEqual(tagsOf(1), ['fresh']);
    assert.deepEqual(tagsOf(2), ['fresh', 'new']);
    assert.deepEqual(tagsOf(3), ['other']);
  });

  test('merges into an existing tag without a duplicate or an error', () => {
    assert.equal(renameTag(db, 'old', 'new'), 2);
    assert.deepEqual(tagsOf(1), ['new']);
    assert.deepEqual(tagsOf(2), ['new']);
    assert.equal(count('task_tags'), 3);
  });

  test('an unknown tag changes nothing', () => {
    assert.equal(renameTag(db, 'missing', 'x'), 0);
    assert.equal(count('task_tags'), 4);
  });

  test('a rename that violates a constraint leaves every task untouched', () => {
    assert.throws(() => renameTag(db, 'old', 'y'.repeat(31)), Database.SqliteError);
    assert.deepEqual(tagsOf(1), ['old']);
    assert.deepEqual(tagsOf(2), ['new', 'old']);
  });
});
