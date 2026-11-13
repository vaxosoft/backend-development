import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import Database from 'better-sqlite3';
import { type Migration, MigrationError, migrateTo, schemaVersion } from './migrations.ts';

const MIGRATIONS: Migration[] = [
  {
    version: 1,
    up: 'CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT)',
    down: 'DROP TABLE tasks',
  },
  {
    version: 2,
    up: 'ALTER TABLE tasks ADD COLUMN due_date TEXT',
    down: 'ALTER TABLE tasks DROP COLUMN due_date',
  },
  { version: 3, up: 'CREATE TABLE tags (task_id INTEGER, tag TEXT)', down: 'DROP TABLE tags' },
];

let db: Database.Database;

beforeEach(() => {
  db = new Database(':memory:');
});

afterEach(() => {
  db.close();
});

const tables = () =>
  db.prepare("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name").pluck().all();
const columns = () => db.prepare('SELECT name FROM pragma_table_info(?)').pluck().all('tasks');

describe('going up', () => {
  test('a new database is at version 0', () => {
    assert.equal(schemaVersion(db), 0);
  });

  test('applies everything by default and reports the range', () => {
    assert.deepEqual(migrateTo(db, MIGRATIONS), { from: 0, to: 3 });
    assert.equal(schemaVersion(db), 3);
    assert.deepEqual(tables(), ['tags', 'tasks']);
    assert.deepEqual(columns(), ['id', 'title', 'due_date']);
  });

  test('is idempotent', () => {
    migrateTo(db, MIGRATIONS);
    assert.deepEqual(migrateTo(db, MIGRATIONS), { from: 3, to: 3 });
  });

  test('stops at the target and resumes later', () => {
    assert.deepEqual(migrateTo(db, MIGRATIONS, 2), { from: 0, to: 2 });
    assert.deepEqual(tables(), ['tasks']);
    assert.deepEqual(migrateTo(db, MIGRATIONS), { from: 2, to: 3 });
  });

  test('migrations are applied in version order whatever the array order', () => {
    const shuffled = [MIGRATIONS[2], MIGRATIONS[0], MIGRATIONS[1]] as Migration[];
    assert.deepEqual(migrateTo(db, shuffled), { from: 0, to: 3 });
    assert.deepEqual(columns(), ['id', 'title', 'due_date']);
  });
});

describe('going down', () => {
  beforeEach(() => {
    migrateTo(db, MIGRATIONS);
  });

  test('runs down steps newest first', () => {
    assert.deepEqual(migrateTo(db, MIGRATIONS, 1), { from: 3, to: 1 });
    assert.deepEqual(tables(), ['tasks']);
    assert.deepEqual(columns(), ['id', 'title']);
  });

  test('target 0 removes everything', () => {
    assert.deepEqual(migrateTo(db, MIGRATIONS, 0), { from: 3, to: 0 });
    assert.deepEqual(tables(), []);
    assert.equal(schemaVersion(db), 0);
  });
});

describe('failures', () => {
  test('a failing step is rolled back, earlier steps stay, and the error names the version', () => {
    const broken: Migration[] = [
      ...MIGRATIONS.slice(0, 2),
      {
        version: 3,
        up: 'CREATE TABLE tags (task_id INTEGER); CREATE TABLE tags (task_id INTEGER)',
        down: 'DROP TABLE tags',
      },
    ];
    assert.throws(
      () => migrateTo(db, broken),
      (err: unknown) => {
        assert.ok(err instanceof MigrationError);
        assert.equal(err.version, 3);
        assert.equal(err.direction, 'up');
        assert.ok(err.cause instanceof Database.SqliteError);
        return true;
      },
    );
    assert.equal(schemaVersion(db), 2);
    assert.deepEqual(tables(), ['tasks']);
    assert.equal(db.inTransaction, false);
  });

  test('a failing down step keeps the version of the last step that worked', () => {
    migrateTo(db, MIGRATIONS);
    const broken = MIGRATIONS.map((m) => (m.version === 2 ? { ...m, down: 'DROP TABLE nope' } : m));
    assert.throws(
      () => migrateTo(db, broken, 0),
      (err: unknown) =>
        err instanceof MigrationError && err.version === 2 && err.direction === 'down',
    );
    assert.equal(schemaVersion(db), 2);
    assert.deepEqual(tables(), ['tasks']);
  });

  test('invalid migration lists are rejected before anything runs', () => {
    const dup: Migration[] = [...MIGRATIONS, { version: 2, up: 'SELECT 1', down: 'SELECT 1' }];
    assert.throws(() => migrateTo(db, dup), RangeError);
    assert.throws(
      () => migrateTo(db, [{ version: 0, up: 'SELECT 1', down: 'SELECT 1' }]),
      RangeError,
    );
    assert.throws(
      () => migrateTo(db, [{ version: 1.5, up: 'SELECT 1', down: 'SELECT 1' }]),
      RangeError,
    );
    assert.deepEqual(tables(), []);
  });

  test('an unknown target is rejected', () => {
    assert.throws(() => migrateTo(db, MIGRATIONS, 7), RangeError);
    assert.throws(() => migrateTo(db, MIGRATIONS, -1), RangeError);
    assert.equal(schemaVersion(db), 0);
  });
});
